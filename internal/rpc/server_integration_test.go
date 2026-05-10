package rpc

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestServerRegistersGRPCHealthOverUnixSocket(t *testing.T) {
	cfg := config.Defaults()
	cfg.GRPCSocket = tempSocketPath(t, "health.sock")

	server, err := NewServer(cfg, newTestService(t), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		if isPermissionError(err) {
			t.Skipf("unix socket is not permitted in this environment: %v", err)
		}
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
		"unix://"+cfg.GRPCSocket,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("dial grpc unix socket: %v", err)
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

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM)
}
