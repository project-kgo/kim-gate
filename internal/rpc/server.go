package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const socketPermission os.FileMode = 0o600

type Server struct {
	server   *grpc.Server
	listener net.Listener
	path     string
	logger   *slog.Logger
	done     chan error
}

func NewServer(cfg config.Config, service *GatewayService, logger *slog.Logger) (*Server, error) {
	listener, err := ListenUnix(cfg.GRPCSocket)
	if err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()
	kimgatev1.RegisterGatewayServiceServer(grpcServer, service)

	healthServer := health.NewServer()
	healthServer.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus(kimgatev1.GatewayService_ServiceDesc.ServiceName, healthpb.HealthCheckResponse_SERVING)
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	return &Server{
		server:   grpcServer,
		listener: listener,
		path:     cfg.GRPCSocket,
		logger:   logger,
		done:     make(chan error, 1),
	}, nil
}

func ListenUnix(socketPath string) (net.Listener, error) {
	socketPath = strings.TrimSpace(socketPath)
	if socketPath == "" {
		return nil, errors.New("grpc socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return nil, fmt.Errorf("create grpc socket dir: %w", err)
	}
	if err := prepareSocketPath(socketPath); err != nil {
		return nil, err
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("listen grpc unix socket: %w", err)
	}
	if err := os.Chmod(socketPath, socketPermission); err != nil {
		_ = listener.Close()
		_ = os.Remove(socketPath)
		return nil, fmt.Errorf("chmod grpc socket: %w", err)
	}
	return listener, nil
}

func (s *Server) Start() {
	go func() {
		if s.logger != nil {
			s.logger.Info("grpc server started", slog.String("socket", s.path))
		}
		err := s.server.Serve(s.listener)
		if errors.Is(err, grpc.ErrServerStopped) {
			err = nil
		}
		s.done <- err
	}()
}

func (s *Server) Done() <-chan error {
	return s.done
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}
	stopped := make(chan struct{})
	go func() {
		s.server.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
	case <-ctx.Done():
		s.server.Stop()
		<-stopped
		return ctx.Err()
	}

	if s.path != "" {
		_ = os.Remove(s.path)
	}
	if s.logger != nil {
		s.logger.Info("grpc server shut down", slog.String("socket", s.path))
	}
	return nil
}

func prepareSocketPath(socketPath string) error {
	info, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat grpc socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("grpc socket path exists and is not a socket: %s", socketPath)
	}

	conn, err := net.DialTimeout("unix", socketPath, 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return fmt.Errorf("grpc socket already in use: %s", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("remove stale grpc socket: %w", err)
	}
	return nil
}
