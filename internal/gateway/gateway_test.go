package gateway

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/project-kgo/signalg"
)

func TestHubOnConnectedRegistersUserRoute(t *testing.T) {
	routes := &fakeUserRoutes{bucket: 7}
	hub := &Hub{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID:   "server-1",
		userRoutes: routes,
	}

	err := hub.OnConnected(context.Background(), &signalg.Connection{
		ID:     "conn-1",
		UserID: "user-1",
	})
	if err != nil {
		t.Fatalf("OnConnected returned error: %v", err)
	}
	if routes.registeredUserID != "user-1" || routes.registeredConnID != "conn-1" {
		t.Fatalf("registered route = (%q, %q)", routes.registeredUserID, routes.registeredConnID)
	}
}

func TestHubOnConnectedDoesNotBlockOnRouteError(t *testing.T) {
	hub := &Hub{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID: "server-1",
		userRoutes: &fakeUserRoutes{
			bucket:      3,
			registerErr: errors.New("redis down"),
		},
	}

	if err := hub.OnConnected(context.Background(), &signalg.Connection{
		ID:     "conn-1",
		UserID: "user-1",
	}); err != nil {
		t.Fatalf("OnConnected returned error: %v", err)
	}
}

type fakeUserRoutes struct {
	bucket           int
	registerErr      error
	registeredUserID string
	registeredConnID string
}

func (f *fakeUserRoutes) RegisterConnection(_ context.Context, userID, connectionID string) error {
	f.registeredUserID = userID
	f.registeredConnID = connectionID
	return f.registerErr
}

func (f *fakeUserRoutes) BucketOf(string) int {
	return f.bucket
}
