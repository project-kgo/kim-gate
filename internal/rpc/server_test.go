package rpc

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/discovery"
)

type noopRegistry struct{}

func (n *noopRegistry) Register(_ context.Context, _ discovery.ServiceInstance) error { return nil }
func (n *noopRegistry) Deregister(_ context.Context) error                            { return nil }
func (n *noopRegistry) Close() error                                                  { return nil }

type spyRegistry struct {
	registered bool
	instance   discovery.ServiceInstance
}

func (s *spyRegistry) Register(_ context.Context, instance discovery.ServiceInstance) error {
	s.registered = true
	s.instance = instance
	return nil
}
func (s *spyRegistry) Deregister(_ context.Context) error { return nil }
func (s *spyRegistry) Close() error                       { return nil }

func TestNewServerTCP(t *testing.T) {
	cfg := config.Defaults()
	cfg.GRPCAddr = "127.0.0.1:0"

	server, err := NewServer(cfg, newTestService(t), slog.New(slog.NewTextHandler(io.Discard, nil)), &noopRegistry{}, "test-instance")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
	}()

	if server.listener == nil {
		t.Fatal("listener is nil")
	}
	if server.listener.Addr().Network() != "tcp" {
		t.Fatalf("network = %q, want tcp", server.listener.Addr().Network())
	}
	if server.instance.ID != "test-instance" {
		t.Fatalf("instance ID = %q, want test-instance", server.instance.ID)
	}
}

func TestNewServerRejectsInvalidAddr(t *testing.T) {
	cfg := config.Defaults()
	cfg.GRPCAddr = "999.999.999.999:99999"

	_, err := NewServer(cfg, newTestService(t), slog.New(slog.NewTextHandler(io.Discard, nil)), &noopRegistry{}, "test")
	if err == nil {
		t.Fatal("expected error for invalid address, got nil")
	}
}

func TestServerRegistersAndDeregisters(t *testing.T) {
	registry := &spyRegistry{}
	cfg := config.Defaults()
	cfg.GRPCAddr = "127.0.0.1:0"

	server, err := NewServer(cfg, newTestService(t), slog.New(slog.NewTextHandler(io.Discard, nil)), registry, "spy-instance")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	server.Start()

	time.Sleep(50 * time.Millisecond)

	if !registry.registered {
		t.Fatal("Register was not called")
	}
	if registry.instance.ID != "spy-instance" {
		t.Fatalf("instance ID = %q, want spy-instance", registry.instance.ID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}
