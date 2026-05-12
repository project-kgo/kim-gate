package data

import "testing"

func TestRedisOptionsFromDSN(t *testing.T) {
	opts, err := redisOptionsFromDSN("redis://:secret@cache.example.com:6380/2")
	if err != nil {
		t.Fatalf("redisOptionsFromDSN returned error: %v", err)
	}
	if opts.Addr != "cache.example.com:6380" {
		t.Fatalf("Addr = %q", opts.Addr)
	}
	if opts.Password != "secret" {
		t.Fatalf("Password = %q", opts.Password)
	}
	if opts.DB != 2 {
		t.Fatalf("DB = %d", opts.DB)
	}
}

func TestRedisOptionsFromDSNRejectsEmptyDSN(t *testing.T) {
	if _, err := redisOptionsFromDSN(" "); err == nil {
		t.Fatal("redisOptionsFromDSN returned nil error")
	}
}
