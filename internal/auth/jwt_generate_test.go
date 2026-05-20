package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGenerateToken_Valid(t *testing.T) {
	resolver := NewJWTResolver("test-secret", 1*time.Hour)

	token, expiresAt, err := resolver.GenerateToken(context.Background(), "myapp", "user123", "ios", "iPhone14,2")
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if expiresAt <= time.Now().Unix() {
		t.Fatal("expires_at is in the past")
	}

	resolved, err := resolver.ResolveToken(context.Background(), token)
	if err != nil {
		t.Fatalf("ResolveToken returned error: %v", err)
	}
	if resolved != "myapp:user123" {
		t.Fatalf("got %q, want myapp:user123", resolved)
	}
}

func TestGenerateToken_EmptySecret(t *testing.T) {
	resolver := NewJWTResolver("", 1*time.Hour)

	_, _, err := resolver.GenerateToken(context.Background(), "myapp", "user123", "", "")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestGenerateToken_EmptyParams(t *testing.T) {
	resolver := NewJWTResolver("test-secret", 1*time.Hour)

	_, _, err := resolver.GenerateToken(context.Background(), "", "user123", "", "")
	if err == nil {
		t.Fatal("expected error for empty app_id")
	}

	_, _, err = resolver.GenerateToken(context.Background(), "myapp", "", "", "")
	if err == nil {
		t.Fatal("expected error for empty user_id")
	}
}
