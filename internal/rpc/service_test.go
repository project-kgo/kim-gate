package rpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"testing"

	"github.com/project-kgo/kim-gate/internal/data"
	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"github.com/project-kgo/signalg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGatewayServiceValidation(t *testing.T) {
	service := newTestService(t)
	publisher := service.publisher.(*stubPushPublisher)

	tests := []struct {
		name string
		call func() error
	}{
		{
			name: "empty method",
			call: func() error {
				_, err := service.Broadcast(context.Background(), &kimgatev1.BroadcastRequest{})
				return err
			},
		},
		{
			name: "empty user ids",
			call: func() error {
				_, err := service.SendToUsers(context.Background(), &kimgatev1.SendToUsersRequest{Method: "server.push"})
				return err
			},
		},
		{
			name: "empty connection ids",
			call: func() error {
				_, err := service.SendToConnections(context.Background(), &kimgatev1.SendToConnectionsRequest{Method: "server.push"})
				return err
			},
		},
		{
			name: "empty close user ids",
			call: func() error {
				_, err := service.CloseUsers(context.Background(), &kimgatev1.CloseUsersRequest{})
				return err
			},
		},
		{
			name: "empty close connection ids",
			call: func() error {
				_, err := service.CloseConnections(context.Background(), &kimgatev1.CloseConnectionsRequest{})
				return err
			},
		},
		{
			name: "user and group both set",
			call: func() error {
				_, err := service.GetOnline(context.Background(), &kimgatev1.GetOnlineRequest{
					UserId: "u1",
					Group:  "g1",
				})
				return err
			},
		},
		{
			name: "empty user id for user connections",
			call: func() error {
				_, err := service.GetUserConnections(context.Background(), &kimgatev1.GetUserConnectionsRequest{})
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.call()
			if status.Code(err) != codes.InvalidArgument {
				t.Fatalf("code = %s, want %s, err=%v", status.Code(err), codes.InvalidArgument, err)
			}
			if publisher.publishCount != 0 {
				t.Fatalf("publish count = %d, want 0", publisher.publishCount)
			}
		})
	}
}

func TestGatewayServicePublishesCloseEvents(t *testing.T) {
	publisher := &stubPushPublisher{}
	service := newTestServiceWithPublisher(t, &stubUserConnectionStore{}, publisher)

	_, err := service.CloseUsers(context.Background(), &kimgatev1.CloseUsersRequest{
		UserIds: []string{" user-1 ", "", "user-2"},
	})
	if err != nil {
		t.Fatalf("CloseUsers returned error: %v", err)
	}
	if publisher.event.GetTarget() != kimgatev1.PushTarget_PUSH_TARGET_CLOSE_USERS {
		t.Fatalf("target = %s, want close users", publisher.event.GetTarget())
	}
	if !reflect.DeepEqual(publisher.event.GetUserIds(), []string{"user-1", "user-2"}) {
		t.Fatalf("user ids = %v", publisher.event.GetUserIds())
	}

	_, err = service.CloseConnections(context.Background(), &kimgatev1.CloseConnectionsRequest{
		ConnectionIds: []string{" conn-1 ", "", "conn-2"},
	})
	if err != nil {
		t.Fatalf("CloseConnections returned error: %v", err)
	}
	if publisher.event.GetTarget() != kimgatev1.PushTarget_PUSH_TARGET_CLOSE_CONNECTIONS {
		t.Fatalf("target = %s, want close connections", publisher.event.GetTarget())
	}
	if !reflect.DeepEqual(publisher.event.GetConnectionIds(), []string{"conn-1", "conn-2"}) {
		t.Fatalf("connection ids = %v", publisher.event.GetConnectionIds())
	}
}

func TestGatewayServicePublishesConnectionEvents(t *testing.T) {
	publisher := &stubPushPublisher{}
	service := newTestServiceWithPublisher(t, &stubUserConnectionStore{}, publisher)

	_, err := service.SendToConnections(context.Background(), &kimgatev1.SendToConnectionsRequest{
		ConnectionIds: []string{" conn-1 ", "", "conn-2", "conn-1"},
		Method:        "server.push",
		Payload:       []byte("connection-payload"),
	})
	if err != nil {
		t.Fatalf("SendToConnections returned error: %v", err)
	}
	if publisher.event.GetTarget() != kimgatev1.PushTarget_PUSH_TARGET_CONNECTIONS {
		t.Fatalf("target = %s, want connections", publisher.event.GetTarget())
	}
	if !reflect.DeepEqual(publisher.event.GetConnectionIds(), []string{"conn-1", "conn-2", "conn-1"}) {
		t.Fatalf("connection ids = %v", publisher.event.GetConnectionIds())
	}
	if publisher.event.GetMethod() != "server.push" {
		t.Fatalf("method = %q, want %q", publisher.event.GetMethod(), "server.push")
	}
	if !reflect.DeepEqual(publisher.event.GetPayload(), []byte("connection-payload")) {
		t.Fatalf("payload = %q", publisher.event.GetPayload())
	}
}

func TestGatewayServicePublishesBroadcastEvent(t *testing.T) {
	publisher := &stubPushPublisher{}
	service := newTestServiceWithPublisher(t, &stubUserConnectionStore{}, publisher)
	payload := []byte("encoded-by-app")

	resp, err := service.Broadcast(context.Background(), &kimgatev1.BroadcastRequest{
		Method:  "server.push",
		Payload: payload,
	})
	if err != nil {
		t.Fatalf("Broadcast returned error: %v", err)
	}
	if resp.Matched != 0 || resp.Sent != 0 || resp.Failed != 0 || resp.Error != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if publisher.publishCount != 1 {
		t.Fatalf("publish count = %d, want 1", publisher.publishCount)
	}
	if publisher.event.GetTarget() != kimgatev1.PushTarget_PUSH_TARGET_BROADCAST || publisher.event.GetMethod() != "server.push" {
		t.Fatalf("event = %+v", publisher.event)
	}
	if !reflect.DeepEqual(publisher.event.GetPayload(), payload) {
		t.Fatalf("payload = %q, want %q", publisher.event.GetPayload(), payload)
	}
}

func TestGatewayServicePublishesUserAndGroupEvents(t *testing.T) {
	publisher := &stubPushPublisher{}
	service := newTestServiceWithPublisher(t, &stubUserConnectionStore{}, publisher)

	_, err := service.SendToUsers(context.Background(), &kimgatev1.SendToUsersRequest{
		UserIds: []string{" user-1 ", "", "user-2"},
		Method:  "server.push",
		Payload: []byte("users-payload"),
	})
	if err != nil {
		t.Fatalf("SendToUsers returned error: %v", err)
	}
	if publisher.event.GetTarget() != kimgatev1.PushTarget_PUSH_TARGET_USERS {
		t.Fatalf("target = %s, want users", publisher.event.GetTarget())
	}
	if !reflect.DeepEqual(publisher.event.GetUserIds(), []string{"user-1", "user-2"}) {
		t.Fatalf("user ids = %v", publisher.event.GetUserIds())
	}
	if !reflect.DeepEqual(publisher.event.GetPayload(), []byte("users-payload")) {
		t.Fatalf("payload = %q", publisher.event.GetPayload())
	}

	_, err = service.SendToGroup(context.Background(), &kimgatev1.SendToGroupRequest{
		Group:   "app:app1",
		Method:  "server.push",
		Payload: []byte("group-payload"),
	})
	if err != nil {
		t.Fatalf("SendToGroup returned error: %v", err)
	}
	if publisher.event.GetTarget() != kimgatev1.PushTarget_PUSH_TARGET_GROUP || publisher.event.GetGroup() != "app:app1" {
		t.Fatalf("event = %+v", publisher.event)
	}
	if !reflect.DeepEqual(publisher.event.GetPayload(), []byte("group-payload")) {
		t.Fatalf("payload = %q", publisher.event.GetPayload())
	}
}

func TestGatewayServicePublishFailureReturnsInternal(t *testing.T) {
	publisher := &stubPushPublisher{err: errors.New("redis down")}
	service := newTestServiceWithPublisher(t, &stubUserConnectionStore{}, publisher)

	_, err := service.Broadcast(context.Background(), &kimgatev1.BroadcastRequest{Method: "server.push"})
	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %s, want %s, err=%v", status.Code(err), codes.Internal, err)
	}
}

func TestGatewayServiceSupportsAppGroupAddressingForOnline(t *testing.T) {
	service := newTestService(t)

	onlineResp, err := service.GetOnline(context.Background(), &kimgatev1.GetOnlineRequest{
		Group: "app:app1",
	})
	if err != nil {
		t.Fatalf("GetOnline returned error: %v", err)
	}
	if onlineResp.Online != 0 {
		t.Fatalf("online = %d, want 0", onlineResp.Online)
	}
}

func TestGatewayServiceGetUserConnections(t *testing.T) {
	store := &stubUserConnectionStore{
		connections: []data.UserConnectionRoute{
			{ConnectionID: "conn-2", ServerID: "server-b"},
			{ConnectionID: "conn-1", ServerID: "server-a"},
		},
	}
	service := newTestServiceWithStore(t, store)

	resp, err := service.GetUserConnections(context.Background(), &kimgatev1.GetUserConnectionsRequest{UserId: "user-1"})
	if err != nil {
		t.Fatalf("GetUserConnections returned error: %v", err)
	}
	if store.userID != "user-1" {
		t.Fatalf("store userID = %q, want %q", store.userID, "user-1")
	}
	want := []*kimgatev1.UserConnection{
		{ConnectionId: "conn-2", ServerId: "server-b"},
		{ConnectionId: "conn-1", ServerId: "server-a"},
	}
	if !reflect.DeepEqual(resp.Connections, want) {
		t.Fatalf("connections = %+v, want %+v", resp.Connections, want)
	}
}

func newTestService(t *testing.T) *GatewayService {
	t.Helper()
	return newTestServiceWithPublisher(t, &stubUserConnectionStore{}, &stubPushPublisher{})
}

func newTestServiceWithStore(t *testing.T, store UserConnectionStore) *GatewayService {
	t.Helper()
	return newTestServiceWithPublisher(t, store, &stubPushPublisher{})
}

func newTestServiceWithPublisher(t *testing.T, store UserConnectionStore, publisher PushPublisher) *GatewayService {
	t.Helper()
	handler, err := signalg.NewHandler(signalg.Config{
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		Serialization: signalg.SerializationProtobuf,
	}, func(*signalg.Connection) (signalg.Hub, error) {
		return &testHub{}, nil
	})
	if err != nil {
		t.Fatalf("NewHandler returned error: %v", err)
	}
	service, err := NewGatewayService(handler, store, publisher)
	if err != nil {
		t.Fatalf("NewGatewayService returned error: %v", err)
	}
	return service
}

type testHub struct{}

func (h *testHub) OnConnected(context.Context, *signalg.Connection) error {
	return nil
}

func (h *testHub) OnDisconnected(context.Context, *signalg.Connection, error) {}

type stubUserConnectionStore struct {
	userID      string
	connections []data.UserConnectionRoute
	err         error
}

func (s *stubUserConnectionStore) ListUserConnections(_ context.Context, userID string) ([]data.UserConnectionRoute, error) {
	s.userID = userID
	return append([]data.UserConnectionRoute(nil), s.connections...), s.err
}

type stubPushPublisher struct {
	publishCount int
	event        *kimgatev1.PushEvent
	err          error
}

func (s *stubPushPublisher) Publish(_ context.Context, event *kimgatev1.PushEvent) error {
	s.publishCount++
	if event != nil {
		copied := *event
		copied.UserIds = append([]string(nil), event.UserIds...)
		copied.ConnectionIds = append([]string(nil), event.ConnectionIds...)
		s.event = &copied
	}
	return s.err
}
