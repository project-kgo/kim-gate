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
	joiner := &fakeGroupJoiner{}
	hub := &Hub{
		logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID:   "server-1",
		userRoutes: routes,
		groupJoiner: joiner,
	}

	err := hub.OnConnected(context.Background(), &signalg.Connection{
		ID:     "conn-1",
		UserID: "app-1:user-1",
	})
	if err != nil {
		t.Fatalf("OnConnected returned error: %v", err)
	}
	if routes.registeredUserID != "app-1:user-1" || routes.registeredConnID != "conn-1" {
		t.Fatalf("registered route = (%q, %q)", routes.registeredUserID, routes.registeredConnID)
	}
	if joiner.calledCount != 1 || joiner.group != "app:app-1" || joiner.connID != "conn-1" {
		t.Fatalf("group join = (%d, %q, %q)", joiner.calledCount, joiner.group, joiner.connID)
	}
}

func TestHubOnConnectedDoesNotBlockOnRouteError(t *testing.T) {
	joiner := &fakeGroupJoiner{}
	hub := &Hub{
		logger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID: "server-1",
		userRoutes: &fakeUserRoutes{
			bucket:      3,
			registerErr: errors.New("redis down"),
		},
		groupJoiner: joiner,
	}

	if err := hub.OnConnected(context.Background(), &signalg.Connection{
		ID:     "conn-1",
		UserID: "app-1:user-1",
	}); err != nil {
		t.Fatalf("OnConnected returned error: %v", err)
	}
	if joiner.calledCount != 1 {
		t.Fatalf("AddToGroup called %d times, want 1", joiner.calledCount)
	}
}

func TestHubOnConnectedRejectsInvalidUserID(t *testing.T) {
	joiner := &fakeGroupJoiner{}
	routes := &fakeUserRoutes{bucket: 1}
	hub := &Hub{
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID:    "server-1",
		userRoutes:  routes,
		groupJoiner: joiner,
	}

	err := hub.OnConnected(context.Background(), &signalg.Connection{
		ID:     "conn-1",
		UserID: "user-1",
	})
	if err == nil {
		t.Fatal("OnConnected error = nil, want error")
	}
	if joiner.calledCount != 0 {
		t.Fatalf("AddToGroup called %d times, want 0", joiner.calledCount)
	}
}

func TestHubOnConnectedReturnsGroupJoinError(t *testing.T) {
	joiner := &fakeGroupJoiner{err: errors.New("join group failed")}
	routes := &fakeUserRoutes{bucket: 5}
	hub := &Hub{
		logger:      slog.New(slog.NewTextHandler(io.Discard, nil)),
		serverID:    "server-1",
		userRoutes:  routes,
		groupJoiner: joiner,
	}

	err := hub.OnConnected(context.Background(), &signalg.Connection{
		ID:     "conn-1",
		UserID: "app-1:user-1",
	})
	if err == nil {
		t.Fatal("OnConnected error = nil, want error")
	}
	if joiner.calledCount != 1 {
		t.Fatalf("AddToGroup called %d times, want 1", joiner.calledCount)
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

type fakeGroupJoiner struct {
	group       string
	connID      string
	err         error
	calledCount int
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

func (f *fakeGroupJoiner) AddToGroup(conn *signalg.Connection, group string) error {
	f.calledCount++
	f.group = group
	if conn != nil {
		f.connID = conn.ID
	}
	return f.err
}
