package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != DefaultHTTPAddr {
		t.Fatalf("HTTPAddr = %q, want %q", cfg.HTTPAddr, DefaultHTTPAddr)
	}
	if cfg.WebSocketPath != DefaultWebSocketPath {
		t.Fatalf("WebSocketPath = %q, want %q", cfg.WebSocketPath, DefaultWebSocketPath)
	}
	if cfg.GRPCAddr != DefaultGRPCAddr {
		t.Fatalf("GRPCAddr = %q, want %q", cfg.GRPCAddr, DefaultGRPCAddr)
	}
	if cfg.ETCDEndpointsStr != DefaultETCDEndpoints {
		t.Fatalf("ETCDEndpoints = %q, want %q", cfg.ETCDEndpointsStr, DefaultETCDEndpoints)
	}
	if len(cfg.ETCDEndpoints) != 1 || cfg.ETCDEndpoints[0] != "localhost:2379" {
		t.Fatalf("ETCDEndpoints = %v, want [localhost:2379]", cfg.ETCDEndpoints)
	}
	if cfg.ETCDServiceName != DefaultETCDServiceName {
		t.Fatalf("ETCDServiceName = %q, want %q", cfg.ETCDServiceName, DefaultETCDServiceName)
	}
	if cfg.ETCDTTL != DefaultETCDTTL {
		t.Fatalf("ETCDTTL = %s, want %s", cfg.ETCDTTL, DefaultETCDTTL)
	}
	if cfg.RedisDSN != DefaultRedisDSN {
		t.Fatalf("RedisDSN = %q, want %q", cfg.RedisDSN, DefaultRedisDSN)
	}
	if cfg.RedisRouteTTL != DefaultRedisRouteTTL {
		t.Fatalf("RedisRouteTTL = %s, want %s", cfg.RedisRouteTTL, DefaultRedisRouteTTL)
	}
	if cfg.RedisPushChannel != DefaultRedisPushChannel {
		t.Fatalf("RedisPushChannel = %q, want %q", cfg.RedisPushChannel, DefaultRedisPushChannel)
	}
	if cfg.RedisPushUsersChannel != DefaultRedisPushChannel+":users" {
		t.Fatalf("RedisPushUsersChannel = %q", cfg.RedisPushUsersChannel)
	}
	if cfg.RedisPushGroupChannel != DefaultRedisPushChannel+":group" {
		t.Fatalf("RedisPushGroupChannel = %q", cfg.RedisPushGroupChannel)
	}
	if cfg.RedisPushBroadcastChannel != DefaultRedisPushChannel+":broadcast" {
		t.Fatalf("RedisPushBroadcastChannel = %q", cfg.RedisPushBroadcastChannel)
	}
	if cfg.JWTSecret != "" {
		t.Fatalf("JWTSecret = %q, want empty", cfg.JWTSecret)
	}
	if cfg.JWTExpiration != 4320*time.Hour {
		t.Fatalf("JWTExpiration = %s, want %s", cfg.JWTExpiration, 4320*time.Hour)
	}
}

func TestLoadYAMLConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(`
http:
  addr: ":9999"
websocket:
  path: "ws"
grpc:
  addr: ":9991"
etcd:
  endpoints: "host1:2379,host2:2379"
  service: "test-gate"
  ttl: "30s"
shutdown:
  timeout: "3s"
signalg:
  ping_interval: "2s"
  ping_timeout: "7s"
redis:
  dsn: "redis://cache.example.com:6380/2"
  route_ttl: "4m"
  push_channel: "custom:push"
  push_users_channel: "custom:users"
  push_group_channel: "custom:group"
  push_broadcast_channel: "custom:broadcast"
jwt:
  secret: "my-jwt-secret"
  expiration: "1h"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load([]string{"-config", path})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":9999" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.WebSocketPath != "/ws" {
		t.Fatalf("WebSocketPath = %q", cfg.WebSocketPath)
	}
	if cfg.GRPCAddr != ":9991" {
		t.Fatalf("GRPCAddr = %q", cfg.GRPCAddr)
	}
	if cfg.ETCDEndpointsStr != "host1:2379,host2:2379" {
		t.Fatalf("ETCDEndpointsStr = %q", cfg.ETCDEndpointsStr)
	}
	if len(cfg.ETCDEndpoints) != 2 || cfg.ETCDEndpoints[0] != "host1:2379" || cfg.ETCDEndpoints[1] != "host2:2379" {
		t.Fatalf("ETCDEndpoints = %v", cfg.ETCDEndpoints)
	}
	if cfg.ETCDServiceName != "test-gate" {
		t.Fatalf("ETCDServiceName = %q", cfg.ETCDServiceName)
	}
	if cfg.ETCDTTL != 30*time.Second {
		t.Fatalf("ETCDTTL = %s", cfg.ETCDTTL)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
	if cfg.PingInterval != 2*time.Second {
		t.Fatalf("PingInterval = %s", cfg.PingInterval)
	}
	if cfg.PingTimeout != 7*time.Second {
		t.Fatalf("PingTimeout = %s", cfg.PingTimeout)
	}
	if cfg.RedisDSN != "redis://cache.example.com:6380/2" {
		t.Fatalf("RedisDSN = %q", cfg.RedisDSN)
	}
	if cfg.RedisRouteTTL != 4*time.Minute {
		t.Fatalf("RedisRouteTTL = %s", cfg.RedisRouteTTL)
	}
	if cfg.RedisPushChannel != "custom:push" {
		t.Fatalf("RedisPushChannel = %q", cfg.RedisPushChannel)
	}
	if cfg.RedisPushUsersChannel != "custom:users" {
		t.Fatalf("RedisPushUsersChannel = %q", cfg.RedisPushUsersChannel)
	}
	if cfg.RedisPushGroupChannel != "custom:group" {
		t.Fatalf("RedisPushGroupChannel = %q", cfg.RedisPushGroupChannel)
	}
	if cfg.RedisPushBroadcastChannel != "custom:broadcast" {
		t.Fatalf("RedisPushBroadcastChannel = %q", cfg.RedisPushBroadcastChannel)
	}
	if cfg.JWTSecret != "my-jwt-secret" {
		t.Fatalf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.JWTExpiration != 1*time.Hour {
		t.Fatalf("JWTExpiration = %s", cfg.JWTExpiration)
	}
}

func TestLoadEnvAndFlagOverride(t *testing.T) {
	t.Setenv("KIM_GATE_HTTP_ADDR", ":9999")
	t.Setenv("KIM_GATE_WS_PATH", "ws")
	t.Setenv("KIM_GATE_GRPC_ADDR", ":9991")
	t.Setenv("KIM_GATE_ETCD_ENDPOINTS", "env1:2379,env2:2379")
	t.Setenv("KIM_GATE_ETCD_SERVICE", "env-gate")
	t.Setenv("KIM_GATE_ETCD_TTL", "45s")
	t.Setenv("KIM_GATE_SHUTDOWN_TIMEOUT", "3s")
	t.Setenv("KIM_GATE_REDIS_DSN", "redis://env.example.com:6379/1")
	t.Setenv("KIM_GATE_REDIS_ROUTE_TTL", "5m")
	t.Setenv("KIM_GATE_REDIS_PUSH_CHANNEL", "env:push")
	t.Setenv("KIM_GATE_REDIS_PUSH_USERS_CHANNEL", "env:users")
	t.Setenv("KIM_GATE_REDIS_PUSH_GROUP_CHANNEL", "env:group")
	t.Setenv("KIM_GATE_REDIS_PUSH_BROADCAST_CHANNEL", "env:broadcast")
	t.Setenv("KIM_GATE_JWT_SECRET", "env-jwt-secret")
	t.Setenv("KIM_GATE_JWT_EXPIRATION", "30m")

	cfg, err := Load([]string{
		"-http-addr", ":7777",
		"-grpc-addr", ":7771",
		"-etcd-endpoints", "flag1:2379",
		"-etcd-service", "flag-gate",
		"-etcd-ttl", "20s",
		"-ping-interval", "2s",
		"-redis-dsn", "redis://flag.example.com:6379/3",
		"-redis-route-ttl", "6m",
		"-redis-push-channel", "flag:push",
		"-redis-push-users-channel", "flag:users",
		"-redis-push-group-channel", "flag:group",
		"-redis-push-broadcast-channel", "flag:broadcast",
	})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":7777" {
		t.Fatalf("HTTPAddr = %q", cfg.HTTPAddr)
	}
	if cfg.WebSocketPath != "/ws" {
		t.Fatalf("WebSocketPath = %q", cfg.WebSocketPath)
	}
	if cfg.GRPCAddr != ":7771" {
		t.Fatalf("GRPCAddr = %q", cfg.GRPCAddr)
	}
	if cfg.ETCDEndpointsStr != "flag1:2379" {
		t.Fatalf("ETCDEndpointsStr = %q", cfg.ETCDEndpointsStr)
	}
	if len(cfg.ETCDEndpoints) != 1 || cfg.ETCDEndpoints[0] != "flag1:2379" {
		t.Fatalf("ETCDEndpoints = %v", cfg.ETCDEndpoints)
	}
	if cfg.ETCDServiceName != "flag-gate" {
		t.Fatalf("ETCDServiceName = %q", cfg.ETCDServiceName)
	}
	if cfg.ETCDTTL != 20*time.Second {
		t.Fatalf("ETCDTTL = %s", cfg.ETCDTTL)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
	if cfg.PingInterval != 2*time.Second {
		t.Fatalf("PingInterval = %s", cfg.PingInterval)
	}
	if cfg.RedisDSN != "redis://flag.example.com:6379/3" {
		t.Fatalf("RedisDSN = %q", cfg.RedisDSN)
	}
	if cfg.RedisRouteTTL != 6*time.Minute {
		t.Fatalf("RedisRouteTTL = %s", cfg.RedisRouteTTL)
	}
	if cfg.RedisPushChannel != "flag:push" {
		t.Fatalf("RedisPushChannel = %q", cfg.RedisPushChannel)
	}
	if cfg.RedisPushUsersChannel != "flag:users" {
		t.Fatalf("RedisPushUsersChannel = %q", cfg.RedisPushUsersChannel)
	}
	if cfg.RedisPushGroupChannel != "flag:group" {
		t.Fatalf("RedisPushGroupChannel = %q", cfg.RedisPushGroupChannel)
	}
	if cfg.RedisPushBroadcastChannel != "flag:broadcast" {
		t.Fatalf("RedisPushBroadcastChannel = %q", cfg.RedisPushBroadcastChannel)
	}
	if cfg.JWTSecret != "env-jwt-secret" {
		t.Fatalf("JWTSecret = %q", cfg.JWTSecret)
	}
	if cfg.JWTExpiration != 30*time.Minute {
		t.Fatalf("JWTExpiration = %s", cfg.JWTExpiration)
	}
}
