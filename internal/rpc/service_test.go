package rpc

import (
	"context"
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
		})
	}
}

func TestGatewayServiceBroadcastMapsSendResult(t *testing.T) {
	service := newTestService(t)

	resp, err := service.Broadcast(context.Background(), &kimgatev1.BroadcastRequest{Method: "server.push"})
	if err != nil {
		t.Fatalf("Broadcast returned error: %v", err)
	}
	if resp.Matched != 0 || resp.Sent != 0 || resp.Failed != 0 || resp.Error != "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestGatewayServiceSupportsAppGroupAddressing(t *testing.T) {
	service := newTestService(t)

	groupResp, err := service.SendToGroup(context.Background(), &kimgatev1.SendToGroupRequest{
		Group:  "app:app1",
		Method: "server.push",
	})
	if err != nil {
		t.Fatalf("SendToGroup returned error: %v", err)
	}
	if groupResp.Matched != 0 || groupResp.Sent != 0 || groupResp.Failed != 0 || groupResp.Error != "" {
		t.Fatalf("unexpected group response: %+v", groupResp)
	}

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
	return newTestServiceWithStore(t, &stubUserConnectionStore{})
}

func newTestServiceWithStore(t *testing.T, store UserConnectionStore) *GatewayService {
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
	service, err := NewGatewayService(handler, store)
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
