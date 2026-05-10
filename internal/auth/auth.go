package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

var ErrUnauthenticated = errors.New("unauthenticated")

type TokenResolver interface {
	ResolveToken(ctx context.Context, token string) (string, error)
}

type RejectResolver struct{}

func NewRejectResolver() *RejectResolver {
	return &RejectResolver{}
}

func (r *RejectResolver) ResolveToken(context.Context, string) (string, error) {
	return "", ErrUnauthenticated
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
	if token := strings.TrimSpace(r.Header.Get("X-Token")); token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("access-token"))
}
