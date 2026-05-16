package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/redis/go-redis/v9"
)

const (
	userRouteBucketCount = 16
	userRouteKeyPrefix   = "kim:gateway:user_route"
	userExpireKeyPrefix  = "kim:gateway:user_expire"
)

var registerConnectionScript = redis.NewScript(`
local connectionID = ARGV[1]
local serverID = ARGV[2]
local ttlSeconds = tonumber(ARGV[3])
local expireScore = tonumber(ARGV[4])
local userID = ARGV[5]

redis.call("HSETEX", KEYS[1], "EX", ttlSeconds, "FIELDS", 1, connectionID, serverID)
redis.call("ZADD", KEYS[2], expireScore, userID)

return 1
`)

var pollExpiredUsersScript = redis.NewScript(`
local limit = tonumber(ARGV[1])
local maxScore = tonumber(ARGV[2])

local userIDs
if limit > 0 then
	userIDs = redis.call("ZRANGE", KEYS[1], "-inf", maxScore, "BYSCORE", "LIMIT", 0, limit)
else
	userIDs = redis.call("ZRANGE", KEYS[1], "-inf", maxScore, "BYSCORE")
end

local count = #userIDs
if count > 0 then
	redis.call("ZREMRANGEBYRANK", KEYS[1], 0, count - 1)
end

return userIDs
`)

type userRouteRedis interface {
	RunScript(ctx context.Context, script *redis.Script, keys []string, args ...interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	PollExpiredUsers(ctx context.Context, key string, max int64, limit int) ([]string, error)
}

type redisUserRouteClient struct {
	client *redis.Client
}

func (c redisUserRouteClient) RunScript(ctx context.Context, script *redis.Script, keys []string, args ...interface{}) error {
	return script.Run(ctx, c.client, keys, args...).Err()
}

func (c redisUserRouteClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return c.client.HGetAll(ctx, key).Result()
}

func (c redisUserRouteClient) PollExpiredUsers(ctx context.Context, key string, max int64, limit int) ([]string, error) {
	return pollExpiredUsersScript.Run(ctx, c.client, []string{key}, limit, max).StringSlice()
}

type UserRouteStore struct {
	redis    userRouteRedis
	logger   *slog.Logger
	routeTTL time.Duration
	serverID string
	now      func() time.Time
}

type UserConnectionRoute struct {
	ConnectionID string
	ServerID     string
}

func NewUserRouteStore(cfg config.Config, data *Data, logger *slog.Logger, serverID string) (*UserRouteStore, error) {
	if data == nil || data.Redis == nil {
		return nil, errors.New("redis client is required")
	}
	return NewUserRouteStoreWithRedis(redisUserRouteClient{client: data.Redis}, cfg.RedisRouteTTL, serverID, logger)
}

func NewUserRouteStoreWithRedis(client userRouteRedis, routeTTL time.Duration, serverID string, logger *slog.Logger) (*UserRouteStore, error) {
	if client == nil {
		return nil, errors.New("user route redis client is required")
	}
	if routeTTL <= 0 {
		return nil, errors.New("route ttl must be positive")
	}
	serverID = strings.TrimSpace(serverID)
	if serverID == "" {
		return nil, errors.New("server id is required")
	}
	return &UserRouteStore{
		redis:    client,
		logger:   logger,
		routeTTL: routeTTL,
		serverID: serverID,
		now:      time.Now,
	}, nil
}

func (s *UserRouteStore) RegisterConnection(ctx context.Context, userID, connectionID string) error {
	return s.refreshConnection(ctx, userID, connectionID)
}

func (s *UserRouteStore) RefreshConnection(ctx context.Context, userID, connectionID string) error {
	return s.refreshConnection(ctx, userID, connectionID)
}

func (s *UserRouteStore) refreshConnection(ctx context.Context, userID, connectionID string) error {
	userID = strings.TrimSpace(userID)
	connectionID = strings.TrimSpace(connectionID)
	if userID == "" {
		return errors.New("user id is required")
	}
	if connectionID == "" {
		return errors.New("connection id is required")
	}

	keys, args := s.registerScriptArgs(userID, connectionID)
	if err := s.redis.RunScript(ctx, registerConnectionScript, keys, args...); err != nil {
		return fmt.Errorf("refresh user route: %w", err)
	}
	return nil
}

func (s *UserRouteStore) ListUserServerIDs(ctx context.Context, userID string) ([]string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	values, err := s.redis.HGetAll(ctx, userRouteKey(s.BucketOf(userID), userID))
	if err != nil {
		return nil, fmt.Errorf("load user route: %w", err)
	}
	if len(values) == 0 {
		return nil, nil
	}

	unique := make(map[string]struct{}, len(values))
	serverIDs := make([]string, 0, len(values))
	for _, serverID := range values {
		serverID = strings.TrimSpace(serverID)
		if serverID == "" {
			continue
		}
		if _, ok := unique[serverID]; ok {
			continue
		}
		unique[serverID] = struct{}{}
		serverIDs = append(serverIDs, serverID)
	}
	sort.Strings(serverIDs)
	return serverIDs, nil
}

func (s *UserRouteStore) ListUserConnections(ctx context.Context, userID string) ([]UserConnectionRoute, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, nil
	}

	values, err := s.redis.HGetAll(ctx, userRouteKey(s.BucketOf(userID), userID))
	if err != nil {
		return nil, fmt.Errorf("load user route: %w", err)
	}
	if len(values) == 0 {
		return nil, nil
	}

	connections := make([]UserConnectionRoute, 0, len(values))
	for connectionID, serverID := range values {
		connectionID = strings.TrimSpace(connectionID)
		serverID = strings.TrimSpace(serverID)
		if connectionID == "" || serverID == "" {
			continue
		}
		connections = append(connections, UserConnectionRoute{
			ConnectionID: connectionID,
			ServerID:     serverID,
		})
	}
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].ConnectionID < connections[j].ConnectionID
	})
	return connections, nil
}

func (s *UserRouteStore) PollExpiredUsers(ctx context.Context, bucket int, limit int, now time.Time, fn func(context.Context, []string) error) error {
	if fn == nil {
		return errors.New("callback is required")
	}
	if bucket < 0 || bucket >= userRouteBucketCount {
		return fmt.Errorf("bucket out of range: %d", bucket)
	}
	if now.IsZero() {
		now = s.now()
	}

	userIDs, err := s.redis.PollExpiredUsers(ctx, userExpireKey(bucket), now.UnixMilli(), limit)
	if err != nil {
		return fmt.Errorf("load expired users: %w", err)
	}
	filtered := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		filtered = append(filtered, userID)
	}
	if len(filtered) == 0 {
		return nil
	}
	return fn(ctx, filtered)
}

func (s *UserRouteStore) BucketOf(userID string) int {
	return bucketOf(userID)
}

func (s *UserRouteStore) registerScriptArgs(userID, connectionID string) ([]string, []any) {
	now := s.now()
	bucket := s.BucketOf(userID)
	keys := []string{
		userRouteKey(bucket, userID),
		userExpireKey(bucket),
	}
	args := []any{
		connectionID,
		s.serverID,
		int64(s.routeTTL / time.Second),
		now.Add(s.routeTTL).UnixMilli(),
		userID,
	}
	return keys, args
}

func bucketOf(userID string) int {
	return int(xxhash.Sum64String(strings.TrimSpace(userID)) % userRouteBucketCount)
}

func UserRouteBucketCount() int {
	return userRouteBucketCount
}

func userRouteKey(bucket int, userID string) string {
	return fmt.Sprintf("%s:{%d}:%s", userRouteKeyPrefix, bucket, userID)
}

func userExpireKey(bucket int) string {
	return fmt.Sprintf("%s:{%d}", userExpireKeyPrefix, bucket)
}
