# JWT Auth Implementation Design

## Summary

Replace the current `RejectResolver` (test-only, auto-increment user IDs) with a `JWTResolver` that validates HMAC-SHA256 signed JWTs and extracts `user_id` + `app_id` claims.

## Configuration

- Add `JWTSecret string` field to `config.Config`
- Populated via environment variable `KIM_GATE_JWT_SECRET` (already covered by `AutomaticEnv`)
- Not written to `config.yml` — env-only
- If empty, JWTResolver returns `ErrUnauthenticated` for all tokens

## New Component: JWTResolver

File: `internal/auth/jwt.go`

```go
type JWTResolver struct {
    secret []byte
}

type KimClaims struct {
    jwt.RegisteredClaims
    UserID string `json:"user_id"`
    AppID  string `json:"app_id"`
}

func NewJWTResolver(secret string) *JWTResolver
func (r *JWTResolver) ResolveToken(ctx context.Context, token string) (string, error)
```

### Behavior

1. Parse token with HMAC-SHA256, validate signature and expiry
2. Extract `user_id` and `app_id` from claims
3. Return formatted string `"appID:userID"` (compatible with existing `ParseAppGroup`)
4. All errors (invalid signature, expired, missing claims) map to `ErrUnauthenticated`

### Dependencies

- `github.com/golang-jwt/jwt/v5`

## DI Changes (wire.go)

```
Replace: auth.NewRejectResolver → auth.NewJWTResolver
Replace: wire.Bind(... *auth.RejectResolver) → wire.Bind(... *auth.JWTResolver)
Add:    ProvideJWTSecret(cfg config.Config) string
```

`RejectResolver` kept for test use.

## Token Format

JWT claims must include:
- `user_id` (string, required)
- `app_id` (string, required)
- `exp` (numeric timestamp, optional — validated if present)

Token extraction unchanged: Bearer header, X-Token header, or `access_token` query param.

## Error Handling

| Scenario | Result |
|----------|--------|
| Empty JWT secret configured | `ErrUnauthenticated` |
| Invalid signature | `ErrUnauthenticated` |
| Expired token | `ErrUnauthenticated` |
| Missing user_id or app_id | `ErrUnauthenticated` |
| Valid token | `"appID:userID"` |

## Testing

- `jwt_test.go`: valid token, expired token, wrong secret, missing claims, empty secret config
