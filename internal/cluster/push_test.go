package cluster

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"github.com/project-kgo/signalg"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

func TestPublisherPublishesProtoPayload(t *testing.T) {
	redisClient := &fakePushRedis{}
	publisher, err := NewPublisherWithRedis(redisClient, "kim:test:push")
	if err != nil {
		t.Fatalf("NewPublisherWithRedis returned error: %v", err)
	}

	event := &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_USERS,
		UserIds: []string{"user-1", "user-2"},
		Method:  "server.push",
		Payload: []byte("payload"),
	}
	if err := publisher.Publish(context.Background(), event); err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	if redisClient.publishChannel != "kim:test:push" {
		t.Fatalf("channel = %q, want %q", redisClient.publishChannel, "kim:test:push")
	}
	raw, ok := redisClient.publishMessage.([]byte)
	if !ok {
		t.Fatalf("message type = %T, want []byte", redisClient.publishMessage)
	}
	var got kimgatev1.PushEvent
	if err := proto.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal published event: %v", err)
	}
	if !proto.Equal(&got, event) {
		t.Fatalf("event = %+v, want %+v", &got, event)
	}
}

func TestSubscriberDispatchesPushEvents(t *testing.T) {
	sender := &fakeLocalSender{}
	subscriber, err := NewSubscriberWithRedis(&fakePushRedis{}, "kim:test:push", sender, discardLogger())
	if err != nil {
		t.Fatalf("NewSubscriberWithRedis returned error: %v", err)
	}

	if err := subscriber.dispatch(context.Background(), &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_USERS,
		UserIds: []string{"user-2", "user-1"},
		Method:  "users.push",
		Payload: []byte("users"),
	}); err != nil {
		t.Fatalf("dispatch users returned error: %v", err)
	}
	if !reflect.DeepEqual(sender.userIDs, []string{"user-2", "user-1"}) || sender.method != "users.push" {
		t.Fatalf("users dispatch = ids %v method %q", sender.userIDs, sender.method)
	}
	if !reflect.DeepEqual(sender.payload, []byte("users")) {
		t.Fatalf("users payload = %q", sender.payload)
	}

	if err := subscriber.dispatch(context.Background(), &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_GROUP,
		Group:   "app:app1",
		Method:  "group.push",
		Payload: []byte("group"),
	}); err != nil {
		t.Fatalf("dispatch group returned error: %v", err)
	}
	if sender.group != "app:app1" || sender.method != "group.push" {
		t.Fatalf("group dispatch = group %q method %q", sender.group, sender.method)
	}
	if !reflect.DeepEqual(sender.payload, []byte("group")) {
		t.Fatalf("group payload = %q", sender.payload)
	}

	if err := subscriber.dispatch(context.Background(), &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_BROADCAST,
		Method:  "all.push",
		Payload: []byte("all"),
	}); err != nil {
		t.Fatalf("dispatch broadcast returned error: %v", err)
	}
	if sender.broadcastMethod != "all.push" {
		t.Fatalf("broadcast method = %q, want %q", sender.broadcastMethod, "all.push")
	}
	if !reflect.DeepEqual(sender.payload, []byte("all")) {
		t.Fatalf("broadcast payload = %q", sender.payload)
	}
}

func TestSubscriberIgnoresBadMessagesAndContinuesUntilContextCancel(t *testing.T) {
	validPayload, err := proto.Marshal(&kimgatev1.PushEvent{
		Target: kimgatev1.PushTarget_PUSH_TARGET_BROADCAST,
		Method: "server.push",
	})
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	subscription := &fakePushSubscription{
		messages: []*redis.Message{
			{Channel: "kim:test:push"},
			{Channel: "kim:test:push", Payload: "not-proto"},
			{Channel: "kim:test:push", Payload: string(validPayload)},
		},
		onEmpty: cancel,
	}
	sender := &fakeLocalSender{}
	subscriber, err := NewSubscriberWithRedis(&fakePushRedis{subscription: subscription}, "kim:test:push", sender, discardLogger())
	if err != nil {
		t.Fatalf("NewSubscriberWithRedis returned error: %v", err)
	}

	if err := subscriber.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("Start returned error: %v", err)
	}
	if subscription.closed != 1 {
		t.Fatalf("subscription closed = %d, want 1", subscription.closed)
	}
	if sender.broadcastMethod != "server.push" {
		t.Fatalf("broadcast method = %q, want %q", sender.broadcastMethod, "server.push")
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type fakePushRedis struct {
	publishChannel string
	publishMessage any
	publishErr     error

	subscribeChannels []string
	subscription      *fakePushSubscription
}

func (f *fakePushRedis) Publish(_ context.Context, channel string, message any) error {
	f.publishChannel = channel
	f.publishMessage = message
	return f.publishErr
}

func (f *fakePushRedis) Subscribe(_ context.Context, channels ...string) pushSubscription {
	f.subscribeChannels = append([]string(nil), channels...)
	if f.subscription != nil {
		return f.subscription
	}
	return &fakePushSubscription{}
}

type fakePushSubscription struct {
	messages []*redis.Message
	onEmpty  func()
	closed   int
}

func (f *fakePushSubscription) ReceiveMessage(ctx context.Context) (*redis.Message, error) {
	if len(f.messages) == 0 {
		if f.onEmpty != nil {
			f.onEmpty()
			f.onEmpty = nil
		}
		<-ctx.Done()
		return nil, ctx.Err()
	}
	msg := f.messages[0]
	f.messages = f.messages[1:]
	return msg, nil
}

func (f *fakePushSubscription) Close() error {
	f.closed++
	return nil
}

type fakeLocalSender struct {
	userIDs         []string
	group           string
	method          string
	broadcastMethod string
	payload         []byte
}

func (f *fakeLocalSender) SendUsers(_ context.Context, userIDs []string, method string, body any) signalg.SendResult {
	f.userIDs = append([]string(nil), userIDs...)
	f.method = method
	f.payload, _ = body.([]byte)
	return signalg.SendResult{Matched: len(userIDs), Sent: len(userIDs)}
}

func (f *fakeLocalSender) SendGroup(_ context.Context, group string, method string, body any) signalg.SendResult {
	f.group = group
	f.method = method
	f.payload, _ = body.([]byte)
	return signalg.SendResult{Matched: 1, Sent: 1}
}

func (f *fakeLocalSender) SendAll(_ context.Context, method string, body any) signalg.SendResult {
	f.broadcastMethod = method
	f.payload, _ = body.([]byte)
	return signalg.SendResult{Matched: 1, Sent: 1}
}
