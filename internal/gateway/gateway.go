package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/cloudwego/hertz/pkg/app"
	hertzserver "github.com/cloudwego/hertz/pkg/app/server"
	"github.com/cloudwego/hertz/pkg/common/utils"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/project-kgo/kim-gate/internal/config"
	"github.com/project-kgo/kim-gate/internal/data"
	"github.com/project-kgo/signalg"
	signalgHz "github.com/project-kgo/signalg/hertz"
)

var ErrMethodNotImplemented = errors.New("method not implemented")

type Hub struct {
	logger      *slog.Logger
	serverID    string
	userRoutes  userRouteRegistrar
	groupJoiner groupJoiner
}

type userRouteRegistrar interface {
	RegisterConnection(ctx context.Context, userID, connectionID string) error
	RefreshConnection(ctx context.Context, userID, connectionID string) error
	BucketOf(userID string) int
}

type groupJoiner interface {
	AddToGroup(conn *signalg.Connection, group string) error
}

type ServerID string

func NewServerID() (ServerID, error) {
	id, err := gonanoid.New()
	if err != nil {
		return "", err
	}
	return ServerID(id), nil
}

func ServerIDString(id ServerID) string {
	return string(id)
}

func NewSignalGHandler(cfg config.Config, logger *slog.Logger, userProvider signalg.UserProvider, userRoutes *data.UserRouteStore, serverID ServerID) (*signalgHz.ManagedHandler, error) {
	var groupHandler *signalg.Handler
	managedHandler, err := signalgHz.NewHandler(signalg.Config{
		Path:          cfg.WebSocketPath,
		Logger:        logger,
		UserProvider:  userProvider,
		Serialization: signalg.SerializationProtobuf,
		PingInterval:  cfg.PingInterval,
		PingTimeout:   cfg.PingTimeout,
	}, func(*signalg.Connection) (signalg.Hub, error) {
		return &Hub{
			logger:      logger,
			serverID:    string(serverID),
			userRoutes:  userRoutes,
			groupJoiner: groupHandler,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	groupHandler = managedHandler.SignalGHandler()
	return managedHandler, nil
}

func SignalGHandler(handler *signalgHz.ManagedHandler) *signalg.Handler {
	if handler == nil {
		return nil
	}
	return handler.SignalGHandler()
}

func NewHertzServer(cfg config.Config, logger *slog.Logger, wsHandler *signalgHz.ManagedHandler) *hertzserver.Hertz {
	h := hertzserver.New(hertzserver.WithHostPorts(cfg.HTTPAddr))
	h.GET("/healthz", func(ctx context.Context, c *app.RequestContext) {
		online := 0
		if wsHandler != nil && wsHandler.SignalGHandler() != nil {
			online = wsHandler.SignalGHandler().Online()
		}
		c.JSON(consts.StatusOK, utils.H{
			"status": "ok",
			"path":   cfg.WebSocketPath,
			"online": online,
		})
	})
	if wsHandler != nil {
		h.GET(cfg.WebSocketPath, wsHandler.Handle)
		h.OnShutdown = append(h.OnShutdown, func(ctx context.Context) {
			if err := wsHandler.Shutdown(ctx); err != nil && logger != nil {
				logger.Error("failed to shutdown signalg handler", slog.Any("error", err))
			}
		})
	}
	return h
}

func (h *Hub) OnConnected(_ context.Context, conn *signalg.Connection) error {
	group, appID, err := ParseAppGroup(conn.UserID)
	if err != nil {
		h.log().Error("invalid websocket user id",
			slog.String("connection_id", conn.ID),
			slog.String("user_id", conn.UserID),
			slog.String("server_id", h.serverID),
			slog.Any("error", err),
		)
		return err
	}

	bucket := 0
	if h.userRoutes != nil {
		bucket = h.userRoutes.BucketOf(conn.UserID)
		if err := h.userRoutes.RegisterConnection(context.Background(), conn.UserID, conn.ID); err != nil {
			h.log().Error("failed to register websocket route",
				slog.String("connection_id", conn.ID),
				slog.String("user_id", conn.UserID),
				slog.String("server_id", h.serverID),
				slog.Int("bucket", bucket),
				slog.Any("error", err),
			)
		}
	}

	if h.groupJoiner != nil {
		if err := h.groupJoiner.AddToGroup(conn, group); err != nil {
			h.log().Error("failed to join websocket app group",
				slog.String("connection_id", conn.ID),
				slog.String("user_id", conn.UserID),
				slog.String("app_id", appID),
				slog.String("group", group),
				slog.String("server_id", h.serverID),
				slog.Any("error", err),
			)
			return err
		}
	}

	h.log().Info("websocket connected",
		slog.String("connection_id", conn.ID),
		slog.String("user_id", conn.UserID),
		slog.String("app_id", appID),
		slog.String("group", group),
		slog.String("server_id", h.serverID),
		slog.Int("bucket", bucket),
		slog.String("remote_addr", remoteAddr(conn)),
	)
	return nil
}

func (h *Hub) OnDisconnected(_ context.Context, conn *signalg.Connection, err error) {
	attrs := []slog.Attr{
		slog.String("connection_id", conn.ID),
		slog.String("user_id", conn.UserID),
		slog.String("server_id", h.serverID),
		slog.String("remote_addr", remoteAddr(conn)),
	}
	if err != nil {
		attrs = append(attrs, slog.Any("error", err))
	}
	h.log().LogAttrs(context.Background(), slog.LevelInfo, "websocket disconnected", attrs...)
}

func (h *Hub) OnPing(ctx context.Context, conn *signalg.Connection) {
	if h.userRoutes != nil {
		bucket := h.userRoutes.BucketOf(conn.UserID)
		if err := h.userRoutes.RefreshConnection(ctx, conn.UserID, conn.ID); err != nil {
			h.log().Error("failed to refresh websocket route",
				slog.String("connection_id", conn.ID),
				slog.String("user_id", conn.UserID),
				slog.String("server_id", h.serverID),
				slog.Int("bucket", bucket),
				slog.Any("error", err),
			)
		}
	}
	h.log().Info("websocket ping received",
		slog.String("connection_id", conn.ID),
		slog.String("user_id", conn.UserID),
		slog.String("server_id", h.serverID),
		slog.String("remote_addr", remoteAddr(conn)),
	)
}

// func (h *Hub) OnMessage(ctx context.Context, conn *signalg.Connection, msg signalg.Message) error {
// 	h.log().Info("websocket message received",
// 		slog.String("connection_id", conn.ID),
// 		slog.String("user_id", conn.UserID),
// 		slog.String("method", msg.Method),
// 		slog.String("kind", msg.Kind.String()),
// 		slog.Int("payload_size", len(msg.Payload)),
// 	)

// 	if msg.Kind == signalg.FrameKindInvoke {
// 		return conn.CompleteError(ctx, msg.InvocationID, ErrMethodNotImplemented)
// 	}
// 	return nil
// }

func (h *Hub) log() *slog.Logger {
	if h != nil && h.logger != nil {
		return h.logger
	}
	return slog.Default()
}

func remoteAddr(conn *signalg.Connection) string {
	if conn == nil || conn.RemoteAddr() == nil {
		return ""
	}
	return conn.RemoteAddr().String()
}

// var _ signalg.MessageHub = (*Hub)(nil)
var _ signalg.Hub = (*Hub)(nil)
var _ signalg.UserProvider = signalg.UserProviderFunc(func(*http.Request) (string, error) { return "", nil })
