package rpc

import (
	"context"
	"io"
	"log/slog"
	"testing"

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

func newTestService(t *testing.T) *GatewayService {
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
	service, err := NewGatewayService(handler)
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
