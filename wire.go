//go:build wireinject
// +build wireinject

package main

import (
	"log/slog"

	"github.com/google/wire"
	"github.com/project-kgo/kim-gate/internal/app"
	"github.com/project-kgo/kim-gate/internal/auth"
	"github.com/project-kgo/kim-gate/internal/cluster"
	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/data"
	etcddisc "github.com/project-kgo/kim-gate/internal/discovery/etcd"
	"github.com/project-kgo/kim-gate/internal/gateway"
	"github.com/project-kgo/kim-gate/internal/rpc"
	"github.com/project-kgo/kim-gate/internal/discovery"
	"github.com/project-kgo/signalg"
)

func Initialize(cfg config.Config, logger *slog.Logger) (*app.App, error) {
	wire.Build(
		ProvideJWTResolver,
		auth.NewUserProvider,
		wire.Bind(new(auth.TokenResolver), new(*auth.JWTResolver)),
		wire.Bind(new(signalg.UserProvider), new(*auth.UserProvider)),
		gateway.NewServerID,
		gateway.ServerIDString,
		data.New,
		data.NewUserRouteStore,
		wire.Bind(new(rpc.UserConnectionStore), new(*data.UserRouteStore)),
		gateway.NewSignalGHandler,
		gateway.SignalGHandler,
		wire.Bind(new(cluster.SignalSender), new(*signalg.Handler)),
		cluster.NewPublisher,
		wire.Bind(new(rpc.PushPublisher), new(*cluster.Publisher)),
		cluster.NewSubscriber,
		gateway.NewHertzServer,
		rpc.NewGatewayService,
		ProvideEtcdConfig,
		etcddisc.New,
		wire.Bind(new(discovery.ServiceRegistry), new(*etcddisc.Registry)),
		rpc.NewServer,
		app.New,
	)
	return nil, nil
}

func ProvideJWTResolver(cfg config.Config) *auth.JWTResolver {
	return auth.NewJWTResolver(cfg.JWTSecret, cfg.JWTExpiration)
}

func ProvideEtcdConfig(cfg config.Config) etcddisc.Config {
	return etcddisc.Config{
		Endpoints:   cfg.ETCDEndpoints,
		ServiceName: cfg.ETCDServiceName,
		Username:    cfg.ETCDUsername,
		Password:    cfg.ETCDPassword,
		TTL:         cfg.ETCDTTL,
	}
}
