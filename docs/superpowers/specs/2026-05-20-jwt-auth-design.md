# JWT Auth Implementation Design

## Summary

Replace the current `RejectResolver` (test-only, auto-increment user IDs) with a `JWTResolver` that validates HMAC-SHA256 signed JWTs and extracts `user_id` + `app_id` claims.

## Configuration

- Add `JWTSecret string` field to `config.Config` — env `KIM_GATE_JWT_SECRET`
- Add `JWTExpiration time.Duration` field to `config.Config` — env `KIM_GATE_JWT_EXPIRATION`
- Both not written to `config.yml` — env-only
- If `JWTSecret` is empty, JWTResolver returns `ErrUnauthenticated` for all tokens
- If `JWTExpiration` is 0, skip the max lifetime check (only validate JWT's own `exp` claim)
- If `JWTExpiration > 0`, reject tokens whose lifetime (`exp - iat`) exceeds it

## New Component: JWTResolver

File: `internal/auth/jwt.go`

```go
type JWTResolver struct {
    secret     []byte
    expiration time.Duration  // max token lifetime (0 = unlimited)
}

type KimClaims struct {
    jwt.RegisteredClaims
    UserID string `json:"user_id"`
    AppID  string `json:"app_id"`
}

func NewJWTResolver(secret string, expiration time.Duration) *JWTResolver
func (r *JWTResolver) ResolveToken(ctx context.Context, token string) (string, error)
```

### Behavior

1. Parse token with HMAC-SHA256, validate signature and standard `exp` claim
2. If `expiration > 0`, validate `exp - iat <= expiration` (reject tokens with excessive lifetime)
3. Extract `user_id` and `app_id` from claims
4. Return formatted string `"appID:userID"` (compatible with existing `ParseAppGroup`)
5. All errors map to `ErrUnauthenticated`

### Dependencies

- `github.com/golang-jwt/jwt/v5`

## DI Changes (wire.go)

```
Replace: auth.NewRejectResolver → auth.NewJWTResolver
Replace: wire.Bind(... *auth.RejectResolver) → wire.Bind(... *auth.JWTResolver)
Add:    ProvideJWTSecret(cfg config.Config) string
Add:    ProvideJWTExpiration(cfg config.Config) time.Duration
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
| Expired token (standard `exp`) | `ErrUnauthenticated` |
| Token lifetime exceeds `JWTExpiration` | `ErrUnauthenticated` |
| Missing `iat` claim when `JWTExpiration > 0` | `ErrUnauthenticated` |
| Missing user_id or app_id | `ErrUnauthenticated` |
| Valid token | `"appID:userID"` |

## Testing

- `jwt_test.go`: valid token, expired token, wrong secret, missing claims, empty secret config, token exceeding max expiration, missing iat with expiration configured
