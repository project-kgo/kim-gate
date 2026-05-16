package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type TokenResolver interface {
	ResolveToken(ctx context.Context, token string) (string, error)
}

type RejectResolver struct{}

func NewRejectResolver() *RejectResolver {
	return &RejectResolver{}
}

var userId = atomic.Int64{}

func (r *RejectResolver) ResolveToken(context.Context, string) (string, error) {
	uid := userId.Add(1)
	return fmt.Sprintf("test:%d", uid), nil
}

type UserProvider struct {
	resolver TokenResolver
}

func NewUserProvider(resolver TokenResolver) *UserProvider {
	return &UserProvider{resolver: resolver}
}

func (p *UserProvider) GetUserID(r *http.Request) (string, error) {
	token := ExtractToken(r)
	if token == "" {
		return "", ErrUnauthenticated
	}
	if p == nil || p.resolver == nil {
		return "", ErrUnauthenticated
	}
	userID, err := p.resolver.ResolveToken(r.Context(), token)
	if err != nil {
		return "", err
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "", ErrUnauthenticated
	}
	return userID, nil
}

func ExtractToken(r *http.Request) string {
	if r == nil {
		return ""
	}
	if token := strings.TrimSpace(r.Header.Get("Authorization")); token != "" {
		if strings.HasPrefix(token, "Bearer ") {
			return token[7:]
		}
		return token
	}
	if token := strings.TrimSpace(r.Header.Get("X-Token")); token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("access_token"))
}
