package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractTokenPrefersHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/signalg?access-token=query-token", nil)
	req.Header.Set("X-Token", " header-token ")

	if got := ExtractToken(req); got != "header-token" {
		t.Fatalf("ExtractToken = %q, want header-token", got)
	}
}

func TestExtractTokenFallsBackToQuery(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/signalg?access_token=query-token", nil)

	if got := ExtractToken(req); got != "query-token" {
		t.Fatalf("ExtractToken = %q, want query-token", got)
	}
}

func TestRejectResolverRejects(t *testing.T) {
	provider := NewUserProvider(NewRejectResolver())
	req := httptest.NewRequest(http.MethodGet, "/signalg?access-token=token", nil)

	_, err := provider.GetUserID(req)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

type resolverFunc func(context.Context, string) (string, error)

func (f resolverFunc) ResolveToken(ctx context.Context, token string) (string, error) {
	return f(ctx, token)
}
