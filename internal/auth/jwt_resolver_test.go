package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func signToken(t *testing.T, secret string, claims KimClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

func TestJWTResolver_ValidToken(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
		UserID: "user123",
		AppID:  "myapp",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver(secret, 2*time.Hour)
	got, err := resolver.ResolveToken(nil, token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "myapp:user123" {
		t.Fatalf("got %q, want myapp:user123", got)
	}
}

func TestJWTResolver_WrongSecret(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
		UserID: "user123",
		AppID:  "myapp",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver("wrong-secret", 0)
	_, err := resolver.ResolveToken(nil, token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestJWTResolver_ExpiredToken(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(-1 * time.Hour)),
		},
		UserID: "user123",
		AppID:  "myapp",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver(secret, 0)
	_, err := resolver.ResolveToken(nil, token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestJWTResolver_MissingClaims(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver(secret, 0)
	_, err := resolver.ResolveToken(nil, token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestJWTResolver_EmptySecret(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
		UserID: "user123",
		AppID:  "myapp",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver("", 0)
	_, err := resolver.ResolveToken(nil, token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestJWTResolver_ExceedsMaxExpiration(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(3 * time.Hour)),
		},
		UserID: "user123",
		AppID:  "myapp",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver(secret, 1*time.Hour)
	_, err := resolver.ResolveToken(nil, token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestJWTResolver_MissingIatWithExpiration(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
		UserID: "user123",
		AppID:  "myapp",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver(secret, 30*time.Minute)
	_, err := resolver.ResolveToken(nil, token)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestJWTResolver_ThroughUserProvider(t *testing.T) {
	secret := "test-secret"
	now := time.Now()
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
		},
		UserID: "user456",
		AppID:  "app2",
	}
	token := signToken(t, secret, claims)

	resolver := NewJWTResolver(secret, 2*time.Hour)
	provider := NewUserProvider(resolver)
	req := httptest.NewRequest(http.MethodGet, "/hub?access_token="+token, nil)

	got, err := provider.GetUserID(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "app2:user456" {
		t.Fatalf("got %q, want app2:user456", got)
	}
}
