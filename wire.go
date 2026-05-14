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
	"github.com/project-kgo/kim-gate/internal/gateway"
	"github.com/project-kgo/kim-gate/internal/rpc"
	"github.com/project-kgo/signalg"
)

func Initialize(cfg config.Config, logger *slog.Logger) (*app.App, error) {
	wire.Build(
		auth.NewRejectResolver,
		auth.NewUserProvider,
		wire.Bind(new(auth.TokenResolver), new(*auth.RejectResolver)),
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
		rpc.NewServer,
		app.New,
	)
	return nil, nil
}
