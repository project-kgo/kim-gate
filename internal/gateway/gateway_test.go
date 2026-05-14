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

func TestHubOnPingRefreshesUserRoute(t *testing.T) {
	routes := &fakeUserRoutes{bucket: 9}
	hub := &Hub{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID:   "server-1",
		userRoutes: routes,
	}

	err := hub.OnPing(context.Background(), &signalg.Connection{
		ID:     "conn-9",
		UserID: "user-9",
	})
	if err != nil {
		t.Fatalf("OnPing returned error: %v", err)
	}
	if routes.refreshedUserID != "user-9" || routes.refreshedConnID != "conn-9" {
		t.Fatalf("refreshed route = (%q, %q)", routes.refreshedUserID, routes.refreshedConnID)
	}
}

type fakeUserRoutes struct {
	bucket           int
	registerErr      error
	registeredUserID string
	registeredConnID string
	refreshedUserID  string
	refreshedConnID  string
}

func (f *fakeUserRoutes) RegisterConnection(_ context.Context, userID, connectionID string) error {
	f.registeredUserID = userID
	f.registeredConnID = connectionID
	return f.registerErr
}

func (f *fakeUserRoutes) RefreshConnection(_ context.Context, userID, connectionID string) error {
	f.refreshedUserID = userID
	f.refreshedConnID = connectionID
	return f.registerErr
}

func (f *fakeUserRoutes) BucketOf(string) int {
	return f.bucket
}
