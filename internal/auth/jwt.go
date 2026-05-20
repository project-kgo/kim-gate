package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTResolver struct {
	secret     []byte
	expiration time.Duration
}

type KimClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	AppID  string `json:"app_id"`
}

func NewJWTResolver(secret string, expiration time.Duration) *JWTResolver {
	return &JWTResolver{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

func (r *JWTResolver) ResolveToken(ctx context.Context, token string) (string, error) {
	if len(r.secret) == 0 {
		return "", ErrUnauthenticated
	}

	claims := &KimClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return r.secret, nil
	})
	if err != nil || !parsed.Valid {
		return "", ErrUnauthenticated
	}

	if r.expiration > 0 {
		if claims.IssuedAt == nil {
			return "", ErrUnauthenticated
		}
		lifetime := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
		if lifetime > r.expiration {
			return "", ErrUnauthenticated
		}
	}

	if claims.UserID == "" || claims.AppID == "" {
		return "", ErrUnauthenticated
	}

	return claims.AppID + ":" + claims.UserID, nil
}
