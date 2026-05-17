package rpc

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/project-kgo/kim-gate/internal/data"
	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"github.com/project-kgo/signalg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UserConnectionStore interface {
	ListUserConnections(ctx context.Context, userID string) ([]data.UserConnectionRoute, error)
}

type PushPublisher interface {
	Publish(ctx context.Context, event *kimgatev1.PushEvent) error
}

type GatewayService struct {
	kimgatev1.UnimplementedGatewayServiceServer
	handler         *signalg.Handler
	connectionStore UserConnectionStore
	publisher       PushPublisher
}

func NewGatewayService(handler *signalg.Handler, connectionStore UserConnectionStore, publisher PushPublisher) (*GatewayService, error) {
	if handler == nil {
		return nil, errors.New("signalg handler is required")
	}
	if connectionStore == nil {
		return nil, errors.New("user connection store is required")
	}
	if publisher == nil {
		return nil, errors.New("push publisher is required")
	}
	return &GatewayService{
		handler:         handler,
		connectionStore: connectionStore,
		publisher:       publisher,
	}, nil
}

func (s *GatewayService) SendToUsers(ctx context.Context, req *kimgatev1.SendToUsersRequest) (*kimgatev1.SendResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateMethod(req.Method); err != nil {
		return nil, err
	}
	userIDs := compactStrings(req.UserIds)
	if len(userIDs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_ids is required")
	}
	event := &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_USERS,
		UserIds: userIDs,
		Method:  strings.TrimSpace(req.Method),
		Payload: req.GetPayload(),
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("publish push event: %v", err))
	}
	return &kimgatev1.SendResponse{}, nil
}

func (s *GatewayService) SendToConnections(ctx context.Context, req *kimgatev1.SendToConnectionsRequest) (*kimgatev1.SendResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateMethod(req.Method); err != nil {
		return nil, err
	}
	connectionIDs := compactStrings(req.ConnectionIds)
	if len(connectionIDs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "connection_ids is required")
	}
	event := &kimgatev1.PushEvent{
		Target:        kimgatev1.PushTarget_PUSH_TARGET_CONNECTIONS,
		ConnectionIds: connectionIDs,
		Method:        strings.TrimSpace(req.Method),
		Payload:       req.GetPayload(),
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("publish push event: %v", err))
	}
	return &kimgatev1.SendResponse{}, nil
}

func (s *GatewayService) SendToGroup(ctx context.Context, req *kimgatev1.SendToGroupRequest) (*kimgatev1.SendResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateMethod(req.Method); err != nil {
		return nil, err
	}
	group := strings.TrimSpace(req.Group)
	if group == "" {
		return nil, status.Error(codes.InvalidArgument, "group is required")
	}
	event := &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_GROUP,
		Group:   group,
		Method:  strings.TrimSpace(req.Method),
		Payload: req.GetPayload(),
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("publish push event: %v", err))
	}
	return &kimgatev1.SendResponse{}, nil
}

func (s *GatewayService) Broadcast(ctx context.Context, req *kimgatev1.BroadcastRequest) (*kimgatev1.SendResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateMethod(req.Method); err != nil {
		return nil, err
	}
	event := &kimgatev1.PushEvent{
		Target:  kimgatev1.PushTarget_PUSH_TARGET_BROADCAST,
		Method:  strings.TrimSpace(req.Method),
		Payload: req.GetPayload(),
	}
	if err := s.publisher.Publish(ctx, event); err != nil {
		return nil, status.Error(codes.Internal, fmt.Sprintf("publish push event: %v", err))
	}
	return &kimgatev1.SendResponse{}, nil
}

func (s *GatewayService) GetOnline(_ context.Context, req *kimgatev1.GetOnlineRequest) (*kimgatev1.GetOnlineResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	userID := strings.TrimSpace(req.UserId)
	group := strings.TrimSpace(req.Group)
	if userID != "" && group != "" {
		return nil, status.Error(codes.InvalidArgument, "user_id and group cannot both be set")
	}
	if userID != "" {
		return &kimgatev1.GetOnlineResponse{Online: int32(s.handler.UserOnline(userID))}, nil
	}
	if group != "" {
		return &kimgatev1.GetOnlineResponse{Online: int32(s.handler.GroupOnline(group))}, nil
	}
	return &kimgatev1.GetOnlineResponse{Online: int32(s.handler.Online())}, nil
}

func (s *GatewayService) GetUserConnections(ctx context.Context, req *kimgatev1.GetUserConnectionsRequest) (*kimgatev1.GetUserConnectionsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}

	userID := strings.TrimSpace(req.UserId)
	if userID == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	routes, err := s.connectionStore.ListUserConnections(ctx, userID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list user connections: %v", err)
	}

	resp := &kimgatev1.GetUserConnectionsResponse{
		Connections: make([]*kimgatev1.UserConnection, 0, len(routes)),
	}
	for _, route := range routes {
		resp.Connections = append(resp.Connections, &kimgatev1.UserConnection{
			ConnectionId: route.ConnectionID,
			ServerId:     route.ServerID,
		})
	}
	return resp, nil
}

func validateMethod(method string) error {
	if strings.TrimSpace(method) == "" {
		return status.Error(codes.InvalidArgument, "method is required")
	}
	return nil
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func sendResponse(result signalg.SendResult) *kimgatev1.SendResponse {
	resp := &kimgatev1.SendResponse{
		Matched: int32(result.Matched),
		Sent:    int32(result.Sent),
		Failed:  int32(result.Failed),
	}
	if result.Err != nil {
		resp.Error = result.Err.Error()
	}
	return resp
}
