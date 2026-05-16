package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/project-kgo/kim-gate/internal/cluster"
	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/data"
	"github.com/project-kgo/kim-gate/internal/rpc"
)

const (
	userRoutePollLimit       = 128
	minUserRoutePollInterval = time.Second
)

type expiredUserRoutePoller interface {
	PollExpiredUsers(ctx context.Context, bucket int, limit int, now time.Time, fn func(context.Context, []string) error) error
}

type App struct {
	cfg            config.Config
	logger         *slog.Logger
	http           *hertzserver.Hertz
	grpc           *rpc.Server
	data           *data.Data
	push           *cluster.Subscriber
	userRoutes     expiredUserRoutePoller
	stopBackground context.CancelFunc
	done           chan error
	once           sync.Once
}

func New(cfg config.Config, logger *slog.Logger, httpServer *hertzserver.Hertz, grpcServer *rpc.Server, data *data.Data, pushSubscriber *cluster.Subscriber, userRoutes *data.UserRouteStore) *App {
	return &App{
		cfg:        cfg,
		logger:     logger,
		http:       httpServer,
		grpc:       grpcServer,
		data:       data,
		push:       pushSubscriber,
		userRoutes: userRoutes,
		done:       make(chan error, 3),
	}
}

func (a *App) Start() error {
	if a == nil {
		return errors.New("app is nil")
	}
	backgroundCtx, cancel := context.WithCancel(context.Background())
	a.stopBackground = cancel
	if a.push != nil {
		go func() {
			if a.logger != nil {
				a.logger.Info("redis push subscriber started", slog.Any("channels", a.push.Channels()))
			}
			if err := a.push.Start(backgroundCtx); err != nil && !errors.Is(err, context.Canceled) {
				a.done <- err
			}
		}()
	}
	if a.userRoutes != nil {
		go a.runUserRoutePoller(backgroundCtx)
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
		if a.stopBackground != nil {
			a.stopBackground()
		}
		httpErr := a.http.Shutdown(ctx)
		grpcErr := a.grpc.Shutdown(ctx)
		dataErr := a.data.Close()
		err = errors.Join(httpErr, grpcErr, dataErr)
	})
	return err
}

func (a *App) runUserRoutePoller(ctx context.Context) {
	if a == nil || a.userRoutes == nil {
		return
	}
	interval := a.userRoutePollInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if a.logger != nil {
		a.logger.Info("user route poller started", slog.Duration("interval", interval), slog.Int("bucket_count", data.UserRouteBucketCount()))
	}

	for {
		a.pollExpiredUserRoutesOnce(ctx, time.Time{})
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *App) pollExpiredUserRoutesOnce(ctx context.Context, now time.Time) {
	if a == nil || a.userRoutes == nil {
		return
	}
	for bucket := 0; bucket < data.UserRouteBucketCount(); bucket++ {
		err := a.userRoutes.PollExpiredUsers(ctx, bucket, userRoutePollLimit, now, func(_ context.Context, userIDs []string) error {
			a.log().Info("expired user routes polled",
				slog.Int("bucket", bucket),
				slog.Int("user_count", len(userIDs)),
				slog.Any("user_ids", userIDs),
			)
			return nil
		})
		if err != nil && !errors.Is(err, context.Canceled) {
			a.log().Error("failed to poll expired user routes",
				slog.Int("bucket", bucket),
				slog.Any("error", err),
			)
		}
	}
}

func (a *App) userRoutePollInterval() time.Duration {
	if a == nil {
		return minUserRoutePollInterval
	}
	interval := a.cfg.RedisRouteTTL / 2
	if interval < minUserRoutePollInterval {
		return minUserRoutePollInterval
	}
	return interval
}

func (a *App) log() *slog.Logger {
	if a != nil && a.logger != nil {
		return a.logger
	}
	return slog.Default()
}
