package cluster

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/data"
	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"github.com/project-kgo/signalg"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

type pushRedis interface {
	Publish(ctx context.Context, channel string, message any) error
	Subscribe(ctx context.Context, channels ...string) pushSubscription
}

type pushSubscription interface {
	ReceiveMessage(ctx context.Context) (*redis.Message, error)
	Close() error
}

type redisPushClient struct {
	client *redis.Client
}

func (c redisPushClient) Publish(ctx context.Context, channel string, message any) error {
	return c.client.Publish(ctx, channel, message).Err()
}

func (c redisPushClient) Subscribe(ctx context.Context, channels ...string) pushSubscription {
	return c.client.Subscribe(ctx, channels...)
}

type SignalSender interface {
	SendUsersRaw(ctx context.Context, userIDs []string, method string, payload []byte) signalg.SendResult
	SendGroupRaw(ctx context.Context, group string, method string, payload []byte) signalg.SendResult
	SendAllRaw(ctx context.Context, method string, payload []byte) signalg.SendResult
}

type pushChannels struct {
	users     string
	group     string
	broadcast string
}

func newPushChannels(cfg config.Config) (pushChannels, error) {
	channels := pushChannels{
		users:     strings.TrimSpace(cfg.RedisPushUsersChannel),
		group:     strings.TrimSpace(cfg.RedisPushGroupChannel),
		broadcast: strings.TrimSpace(cfg.RedisPushBroadcastChannel),
	}
	return normalizePushChannels(channels)
}

func normalizePushChannels(channels pushChannels) (pushChannels, error) {
	channels.users = strings.TrimSpace(channels.users)
	channels.group = strings.TrimSpace(channels.group)
	channels.broadcast = strings.TrimSpace(channels.broadcast)
	if channels.users == "" {
		return pushChannels{}, errors.New("users push channel is required")
	}
	if channels.group == "" {
		return pushChannels{}, errors.New("group push channel is required")
	}
	if channels.broadcast == "" {
		return pushChannels{}, errors.New("broadcast push channel is required")
	}
	return channels, nil
}

func (c pushChannels) list() []string {
	return compactStrings([]string{c.users, c.group, c.broadcast})
}

func (c pushChannels) uniqueList() []string {
	channels := c.list()
	if len(channels) <= 1 {
		return channels
	}
	out := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		if _, ok := seen[channel]; ok {
			continue
		}
		seen[channel] = struct{}{}
		out = append(out, channel)
	}
	return out
}

func (c pushChannels) channelForTarget(target kimgatev1.PushTarget) (string, error) {
	switch target {
	case kimgatev1.PushTarget_PUSH_TARGET_USERS:
		return c.users, nil
	case kimgatev1.PushTarget_PUSH_TARGET_GROUP:
		return c.group, nil
	case kimgatev1.PushTarget_PUSH_TARGET_BROADCAST:
		return c.broadcast, nil
	default:
		return "", fmt.Errorf("unknown push target: %s", target)
	}
}

type Publisher struct {
	redis    pushRedis
	channels pushChannels
}

func NewPublisher(cfg config.Config, data *data.Data) (*Publisher, error) {
	if data == nil || data.Redis == nil {
		return nil, errors.New("redis client is required")
	}
	channels, err := newPushChannels(cfg)
	if err != nil {
		return nil, err
	}
	return NewPublisherWithRedis(redisPushClient{client: data.Redis}, channels)
}

func NewPublisherWithRedis(redisClient pushRedis, channels pushChannels) (*Publisher, error) {
	if redisClient == nil {
		return nil, errors.New("push redis client is required")
	}
	normalized, err := normalizePushChannels(channels)
	if err != nil {
		return nil, err
	}
	return &Publisher{redis: redisClient, channels: normalized}, nil
}

func (p *Publisher) Publish(ctx context.Context, event *kimgatev1.PushEvent) error {
	if p == nil {
		return errors.New("publisher is nil")
	}
	if event == nil {
		return errors.New("push event is required")
	}
	channel, err := p.channels.channelForTarget(event.GetTarget())
	if err != nil {
		return err
	}
	payload, err := proto.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal push event: %w", err)
	}
	if err := p.redis.Publish(ctx, channel, payload); err != nil {
		return fmt.Errorf("publish push event: %w", err)
	}
	return nil
}

type Subscriber struct {
	redis    pushRedis
	channels pushChannels
	sender   SignalSender
	logger   *slog.Logger
}

func NewSubscriber(cfg config.Config, data *data.Data, sender SignalSender, logger *slog.Logger) (*Subscriber, error) {
	if data == nil || data.Redis == nil {
		return nil, errors.New("redis client is required")
	}
	channels, err := newPushChannels(cfg)
	if err != nil {
		return nil, err
	}
	return NewSubscriberWithRedis(redisPushClient{client: data.Redis}, channels, sender, logger)
}

func NewSubscriberWithRedis(redisClient pushRedis, channels pushChannels, sender SignalSender, logger *slog.Logger) (*Subscriber, error) {
	if redisClient == nil {
		return nil, errors.New("push redis client is required")
	}
	normalized, err := normalizePushChannels(channels)
	if err != nil {
		return nil, err
	}
	if sender == nil {
		return nil, errors.New("local sender is required")
	}
	return &Subscriber{
		redis:    redisClient,
		channels: normalized,
		sender:   sender,
		logger:   logger,
	}, nil
}

func (s *Subscriber) Start(ctx context.Context) error {
	if s == nil {
		return errors.New("subscriber is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	subscribeChannels := s.Channels()
	subscription := s.redis.Subscribe(ctx, subscribeChannels...)
	defer func() {
		if err := subscription.Close(); err != nil {
			s.log().Error("failed to close push subscription", slog.Any("error", err))
		}
	}()

	for {
		msg, err := subscription.ReceiveMessage(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			s.log().Error("failed to receive push event", slog.Any("channels", subscribeChannels), slog.Any("error", err))
			continue
		}
		if msg == nil || msg.Payload == "" {
			s.log().Warn("empty push event ignored", slog.String("channel", msgChannel(msg)))
			continue
		}
		var event kimgatev1.PushEvent
		if err := proto.Unmarshal([]byte(msg.Payload), &event); err != nil {
			s.log().Error("invalid push event ignored", slog.String("channel", msg.Channel), slog.Any("error", err))
			continue
		}
		if err := s.dispatch(ctx, &event); err != nil {
			s.log().Error("failed to dispatch push event",
				slog.String("channel", msg.Channel),
				slog.String("target", event.GetTarget().String()),
				slog.String("method", event.GetMethod()),
				slog.Any("error", err),
			)
		}
	}
}

func (s *Subscriber) dispatch(ctx context.Context, event *kimgatev1.PushEvent) error {
	if event == nil {
		return errors.New("push event is required")
	}
	method := strings.TrimSpace(event.GetMethod())
	if method == "" {
		return errors.New("method is required")
	}
	payload := event.GetPayload()
	switch event.GetTarget() {
	case kimgatev1.PushTarget_PUSH_TARGET_USERS:
		userIDs := compactStrings(event.GetUserIds())
		if len(userIDs) == 0 {
			return errors.New("user_ids is required")
		}
		result := s.sender.SendUsersRaw(ctx, userIDs, method, payload)
		return result.Err
	case kimgatev1.PushTarget_PUSH_TARGET_GROUP:
		group := strings.TrimSpace(event.GetGroup())
		if group == "" {
			return errors.New("group is required")
		}
		result := s.sender.SendGroupRaw(ctx, group, method, payload)
		return result.Err
	case kimgatev1.PushTarget_PUSH_TARGET_BROADCAST:
		result := s.sender.SendAllRaw(ctx, method, payload)
		return result.Err
	default:
		return fmt.Errorf("unknown push target: %s", event.GetTarget())
	}
}

func (s *Subscriber) Channels() []string {
	if s == nil {
		return nil
	}
	return s.channels.uniqueList()
}

func (s *Subscriber) log() *slog.Logger {
	if s != nil && s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

func msgChannel(msg *redis.Message) string {
	if msg == nil {
		return ""
	}
	return msg.Channel
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
