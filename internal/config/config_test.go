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
	if cfg.GRPCSocket != DefaultGRPCSocket {
		t.Fatalf("GRPCSocket = %q, want %q", cfg.GRPCSocket, DefaultGRPCSocket)
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
  socket: "/tmp/yaml.sock"
shutdown:
  timeout: "3s"
signalg:
  ping_interval: "2s"
  ping_timeout: "7s"
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
	if cfg.GRPCSocket != "/tmp/yaml.sock" {
		t.Fatalf("GRPCSocket = %q", cfg.GRPCSocket)
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
}

func TestLoadEnvAndFlagOverride(t *testing.T) {
	t.Setenv("KIM_GATE_HTTP_ADDR", ":9999")
	t.Setenv("KIM_GATE_WS_PATH", "ws")
	t.Setenv("KIM_GATE_GRPC_SOCKET", "/tmp/env.sock")
	t.Setenv("KIM_GATE_SHUTDOWN_TIMEOUT", "3s")

	cfg, err := Load([]string{
		"-http-addr", ":7777",
		"-grpc-socket", "/tmp/flag.sock",
		"-ping-interval", "2s",
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
	if cfg.GRPCSocket != "/tmp/flag.sock" {
		t.Fatalf("GRPCSocket = %q", cfg.GRPCSocket)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout = %s", cfg.ShutdownTimeout)
	}
	if cfg.PingInterval != 2*time.Second {
		t.Fatalf("PingInterval = %s", cfg.PingInterval)
	}
}
