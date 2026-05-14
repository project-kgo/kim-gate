package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/project-kgo/kim-gate/internal/cluster"
	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/data"
	"github.com/project-kgo/kim-gate/internal/rpc"
)

type App struct {
	cfg        config.Config
	logger     *slog.Logger
	http       *hertzserver.Hertz
	grpc       *rpc.Server
	data       *data.Data
	push       *cluster.Subscriber
	pushCancel context.CancelFunc
	done       chan error
	once       sync.Once
}

func New(cfg config.Config, logger *slog.Logger, httpServer *hertzserver.Hertz, grpcServer *rpc.Server, data *data.Data, pushSubscriber *cluster.Subscriber) *App {
	return &App{
		cfg:    cfg,
		logger: logger,
		http:   httpServer,
		grpc:   grpcServer,
		data:   data,
		push:   pushSubscriber,
		done:   make(chan error, 3),
	}
}

func (a *App) Start() error {
	if a == nil {
		return errors.New("app is nil")
	}
	if a.push != nil {
		ctx, cancel := context.WithCancel(context.Background())
		a.pushCancel = cancel
		go func() {
			if a.logger != nil {
				a.logger.Info("redis push subscriber started", slog.String("channel", a.cfg.RedisPushChannel))
			}
			if err := a.push.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
				a.done <- err
			}
		}()
	}
	a.grpc.Start()
	go func() {
		if a.logger != nil {
			a.logger.Info("hertz server started",
				slog.String("addr", a.cfg.HTTPAddr),
				slog.String("websocket_path", a.cfg.WebSocketPath),
			)
		}
		a.done <- a.http.Run()
	}()
	go func() {
		if err := <-a.grpc.Done(); err != nil {
			a.done <- err
		}
	}()
	return nil
}

func (a *App) Done() <-chan error {
	return a.done
}

func (a *App) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}
	var err error
	a.once.Do(func() {
		if a.pushCancel != nil {
			a.pushCancel()
		}
		httpErr := a.http.Shutdown(ctx)
		grpcErr := a.grpc.Shutdown(ctx)
		dataErr := a.data.Close()
		err = errors.Join(httpErr, grpcErr, dataErr)
	})
	return err
}
