package rpc

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/kanengo/ku/ipx"
	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/discovery"
	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type Server struct {
	server   *grpc.Server
	listener net.Listener
	addr     string
	registry discovery.ServiceRegistry
	instance discovery.ServiceInstance
	logger   *slog.Logger
	done     chan error
}

func NewServer(cfg config.Config, service *GatewayService, logger *slog.Logger, registry discovery.ServiceRegistry, instanceID string) (*Server, error) {
	listener, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		return nil, fmt.Errorf("listen grpc tcp %s: %w", cfg.GRPCAddr, err)
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
		addr:     listener.Addr().String(),
		registry: registry,
		instance: discovery.ServiceInstance{
			ID:      instanceID,
			Name:    etcdServiceName(cfg),
			Address: etcdInternalAddr(cfg),
		},
		logger: logger,
		done:   make(chan error, 1),
	}, nil
}

func (s *Server) Start() {
	go func() {
		if s.logger != nil {
			s.logger.Info("grpc server started", slog.String("addr", s.addr))
		}
		err := s.server.Serve(s.listener)
		if errors.Is(err, grpc.ErrServerStopped) {
			err = nil
		}
		s.done <- err
	}()

	if s.registry != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := s.registry.Register(ctx, s.instance); err != nil && s.logger != nil {
				s.logger.Error("failed to register with service registry",
					slog.Any("error", err),
					slog.String("service", s.instance.Name),
					slog.String("instance", s.instance.ID),
				)
			}
		}()
	}
}

func (s *Server) Done() <-chan error {
	return s.done
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.server == nil {
		return nil
	}

	if s.registry != nil {
		if err := s.registry.Deregister(ctx); err != nil && s.logger != nil {
			s.logger.Error("failed to deregister from service registry", slog.Any("error", err))
		}
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

	if s.registry != nil {
		if err := s.registry.Close(); err != nil && s.logger != nil {
			s.logger.Error("failed to close service registry", slog.Any("error", err))
		}
	}

	if s.logger != nil {
		s.logger.Info("grpc server shut down", slog.String("addr", s.addr))
	}
	return nil
}

func etcdInternalAddr(cfg config.Config) string {
	_, port, err := net.SplitHostPort(cfg.GRPCAddr)
	if err != nil {
		panic(err)
	}
	internalIp, err := ipx.GetInternalIp()
	if err != nil {
		panic(err)
	}

	return net.JoinHostPort(internalIp, port)
}

func etcdServiceName(cfg config.Config) string {
	if cfg.Env == "" {
		return cfg.ETCDServiceName
	}
	return cfg.ETCDServiceName + "-" + cfg.Env
}
