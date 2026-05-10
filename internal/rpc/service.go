package rpc

import (
	"context"
	"errors"
	"strings"

	kimgatev1 "github.com/project-kgo/kim-gate/proto/kimgate/v1"
	"github.com/project-kgo/signalg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/anypb"
)

type GatewayService struct {
	kimgatev1.UnimplementedGatewayServiceServer
	handler *signalg.Handler
}

func NewGatewayService(handler *signalg.Handler) (*GatewayService, error) {
	if handler == nil {
		return nil, errors.New("signalg handler is required")
	}
	return &GatewayService{handler: handler}, nil
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
	return sendResponse(s.handler.SendUsers(ctx, userIDs, req.Method, payloadOrEmpty(req.Payload))), nil
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
	return sendResponse(s.handler.SendGroup(ctx, group, req.Method, payloadOrEmpty(req.Payload))), nil
}

func (s *GatewayService) Broadcast(ctx context.Context, req *kimgatev1.BroadcastRequest) (*kimgatev1.SendResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if err := validateMethod(req.Method); err != nil {
		return nil, err
	}
	return sendResponse(s.handler.SendAll(ctx, req.Method, payloadOrEmpty(req.Payload))), nil
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

func payloadOrEmpty(payload *anypb.Any) *anypb.Any {
	if payload != nil {
		return payload
	}
	return &anypb.Any{}
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
