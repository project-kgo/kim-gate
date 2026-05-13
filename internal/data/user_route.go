package data

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
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
local hashKey = KEYS[1]
local zsetKey = KEYS[2]
local connectionID = ARGV[1]
local serverID = ARGV[2]
local ttlSeconds = tonumber(ARGV[3])
local expireScore = tonumber(ARGV[4])
local userID = ARGV[5]

redis.call("HSETEX", hashKey, "EX", ttlSeconds, "FIELDS", 1, connectionID, serverID)
redis.call("ZADD", zsetKey, expireScore, userID)

return 1
`)

type userRouteRedis interface {
	RunScript(ctx context.Context, script *redis.Script, keys []string, args ...interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	ZRangeByScore(ctx context.Context, key string, max int64, limit int) ([]string, error)
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

func (c redisUserRouteClient) ZRangeByScore(ctx context.Context, key string, max int64, limit int) ([]string, error) {
	args := redis.ZRangeArgs{
		Key:     key,
		Start:   "-inf",
		Stop:    strconv.FormatInt(max, 10),
		ByScore: true,
	}
	if limit > 0 {
		args.Count = int64(limit)
	}
	return c.client.ZRangeArgs(ctx, args).Result()
}

type UserRouteStore struct {
	redis    userRouteRedis
	logger   *slog.Logger
	routeTTL time.Duration
	serverID string
	now      func() time.Time
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
		return fmt.Errorf("register user route: %w", err)
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

func (s *UserRouteStore) PollExpiredUsers(ctx context.Context, bucket int, limit int, now time.Time, fn func(context.Context, string) error) error {
	if fn == nil {
		return errors.New("callback is required")
	}
	if bucket < 0 || bucket >= userRouteBucketCount {
		return fmt.Errorf("bucket out of range: %d", bucket)
	}
	if now.IsZero() {
		now = s.now()
	}

	userIDs, err := s.redis.ZRangeByScore(ctx, userExpireKey(bucket), now.UnixMilli(), limit)
	if err != nil {
		return fmt.Errorf("load expired users: %w", err)
	}
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		if err := fn(ctx, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s *UserRouteStore) BucketOf(userID string) int {
	return bucketOf(userID)
}

func (s *UserRouteStore) registerScriptArgs(userID, connectionID string) ([]string, []interface{}) {
	now := s.now()
	bucket := s.BucketOf(userID)
	keys := []string{
		userRouteKey(bucket, userID),
		userExpireKey(bucket),
	}
	args := []interface{}{
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

func userRouteKey(bucket int, userID string) string {
	return fmt.Sprintf("%s:{%d}:%s", userRouteKeyPrefix, bucket, userID)
}

func userExpireKey(bucket int) string {
	return fmt.Sprintf("%s:{%d}", userExpireKeyPrefix, bucket)
}
