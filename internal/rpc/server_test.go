package rpc

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestListenUnixRemovesStaleSocket(t *testing.T) {
	path := tempSocketPath(t, "stale.sock")
	stale := listenUnixForTest(t, path)
	if err := stale.Close(); err != nil {
		t.Fatalf("close stale socket: %v", err)
	}

	listener, err := ListenUnix(path)
	if err != nil {
		t.Fatalf("ListenUnix returned error: %v", err)
	}
	defer listener.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat socket: %v", err)
	}
	if got := info.Mode().Perm(); got != socketPermission {
		t.Fatalf("socket permission = %v, want %v", got, socketPermission)
	}
}

func TestListenUnixRejectsNonSocketFile(t *testing.T) {
	path := tempSocketPath(t, "not-socket")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if listener, err := ListenUnix(path); err == nil {
		listener.Close()
		t.Fatal("expected error")
	}
}

func TestListenUnixRejectsActiveSocket(t *testing.T) {
	path := tempSocketPath(t, "active.sock")
	active := listenUnixForTest(t, path)
	defer active.Close()

	if listener, err := ListenUnix(path); err == nil {
		listener.Close()
		t.Fatal("expected error")
	}
}

func tempSocketPath(t *testing.T, name string) string {
	t.Helper()

	dir, err := os.MkdirTemp("/private/tmp", "kg-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return filepath.Join(dir, name)
}

func listenUnixForTest(t *testing.T, path string) net.Listener {
	t.Helper()

	listener, err := net.Listen("unix", path)
	if err != nil {
		if errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EPERM) {
			t.Skipf("unix socket is not permitted in this environment: %v", err)
		}
		t.Fatalf("listen unix socket: %v", err)
	}
	return listener
}
