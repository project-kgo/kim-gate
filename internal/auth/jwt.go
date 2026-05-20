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
	UserID   string `json:"user_id"`
	AppID    string `json:"app_id"`
	Platform string `json:"platform,omitempty"`
	Br       string `json:"br,omitempty"`
}

func NewJWTResolver(secret string, expiration time.Duration) *JWTResolver {
	return &JWTResolver{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

func (r *JWTResolver) GenerateToken(_ context.Context, appID, userID, platform, br string) (string, int64, error) {
	if len(r.secret) == 0 {
		return "", 0, ErrUnauthenticated
	}
	if appID == "" || userID == "" {
		return "", 0, fmt.Errorf("app_id and user_id are required")
	}
	now := time.Now()
	exp := now.Add(r.expiration)
	claims := KimClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
		UserID:   userID,
		AppID:    appID,
		Platform: platform,
		Br:       br,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(r.secret)
	if err != nil {
		return "", 0, fmt.Errorf("sign token: %w", err)
	}
	return signed, exp.Unix(), nil
}

func (r *JWTResolver) ResolveToken(ctx context.Context, token string) (string, error) {
	if len(r.secret) == 0 {
		return "", ErrUnauthenticated
	}

	claims := &KimClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return r.secret, nil
	})
	if err != nil || !parsed.Valid {
		return "", ErrUnauthenticated
	}

	if r.expiration > 0 {
		if claims.IssuedAt == nil || claims.ExpiresAt == nil {
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
