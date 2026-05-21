package rpc

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestServerRegistersGRPCHealth(t *testing.T) {
	cfg := config.Defaults()
	cfg.GRPCAddr = "127.0.0.1:0"

	server, err := NewServer(cfg, newTestService(t), slog.New(slog.NewTextHandler(io.Discard, nil)), &noopRegistry{}, "integ-test-instance")
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	server.Start()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Fatalf("Shutdown returned error: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		ctx,
		server.listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc: %v", err)
	}
	defer conn.Close()

	resp, err := healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		t.Fatalf("health check returned error: %v", err)
	}
	if resp.Status != healthpb.HealthCheckResponse_SERVING {
		t.Fatalf("health status = %s, want SERVING", resp.Status)
	}
}
