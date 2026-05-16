package data

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestNewUserRouteStoreRequiresInputs(t *testing.T) {
	if _, err := NewUserRouteStoreWithRedis(nil, time.Minute, "server-1", nil); err == nil {
		t.Fatal("NewUserRouteStoreWithRedis returned nil error for nil client")
	}
	if _, err := NewUserRouteStoreWithRedis(&fakeUserRouteRedis{}, 0, "server-1", nil); err == nil {
		t.Fatal("NewUserRouteStoreWithRedis returned nil error for ttl")
	}
	if _, err := NewUserRouteStoreWithRedis(&fakeUserRouteRedis{}, time.Minute, " ", nil); err == nil {
		t.Fatal("NewUserRouteStoreWithRedis returned nil error for server id")
	}
}

func TestUserRouteStoreRegisterConnectionBuildsScriptArgs(t *testing.T) {
	client := &fakeUserRouteRedis{}
	store, err := NewUserRouteStoreWithRedis(client, 2*time.Minute, "server-a", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("NewUserRouteStoreWithRedis returned error: %v", err)
	}
	fixedNow := time.UnixMilli(1_700_000_000_000)
	store.now = func() time.Time { return fixedNow }

	if err := store.RegisterConnection(context.Background(), "user-1", "conn-1"); err != nil {
		t.Fatalf("RegisterConnection returned error: %v", err)
	}

	wantBucket := store.BucketOf("user-1")
	wantKeys := []string{
		userRouteKey(wantBucket, "user-1"),
		userExpireKey(wantBucket),
	}
	if !reflect.DeepEqual(client.runKeys, wantKeys) {
		t.Fatalf("script keys = %v, want %v", client.runKeys, wantKeys)
	}
	wantArgs := []interface{}{
		"conn-1",
		"server-a",
		int64(120),
		fixedNow.Add(2 * time.Minute).UnixMilli(),
		"user-1",
	}
	if !reflect.DeepEqual(client.runArgs, wantArgs) {
		t.Fatalf("script args = %#v, want %#v", client.runArgs, wantArgs)
	}
	if client.runScript == nil || client.runScript.Hash() != registerConnectionScript.Hash() {
		t.Fatal("script hash mismatch")
	}
}

func TestUserRouteStoreRefreshConnectionUsesSameLuaScript(t *testing.T) {
	client := &fakeUserRouteRedis{}
	store, err := NewUserRouteStoreWithRedis(client, 90*time.Second, "server-a", nil)
	if err != nil {
		t.Fatalf("NewUserRouteStoreWithRedis returned error: %v", err)
	}
	fixedNow := time.UnixMilli(1_800_000_000_000)
	store.now = func() time.Time { return fixedNow }

	if err := store.RefreshConnection(context.Background(), "user-2", "conn-2"); err != nil {
		t.Fatalf("RefreshConnection returned error: %v", err)
	}

	wantBucket := store.BucketOf("user-2")
	wantKeys := []string{
		userRouteKey(wantBucket, "user-2"),
		userExpireKey(wantBucket),
	}
	if !reflect.DeepEqual(client.runKeys, wantKeys) {
		t.Fatalf("script keys = %v, want %v", client.runKeys, wantKeys)
	}
	wantArgs := []interface{}{
		"conn-2",
		"server-a",
		int64(90),
		fixedNow.Add(90 * time.Second).UnixMilli(),
		"user-2",
	}
	if !reflect.DeepEqual(client.runArgs, wantArgs) {
		t.Fatalf("script args = %#v, want %#v", client.runArgs, wantArgs)
	}
	if client.runScript == nil || client.runScript.Hash() != registerConnectionScript.Hash() {
		t.Fatal("script hash mismatch")
	}
}

func TestUserRouteStoreListUserServerIDsDeduplicates(t *testing.T) {
	client := &fakeUserRouteRedis{
		hashValues: map[string]string{
			"conn-2": "server-b",
			"conn-1": "server-a",
			"conn-3": "server-a",
			"conn-4": " ",
		},
	}
	store, err := NewUserRouteStoreWithRedis(client, time.Minute, "server-x", nil)
	if err != nil {
		t.Fatalf("NewUserRouteStoreWithRedis returned error: %v", err)
	}

	got, err := store.ListUserServerIDs(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListUserServerIDs returned error: %v", err)
	}
	want := []string{"server-a", "server-b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ListUserServerIDs = %v, want %v", got, want)
	}
}

func TestUserRouteStoreListUserConnections(t *testing.T) {
	client := &fakeUserRouteRedis{
		hashValues: map[string]string{
			"conn-2": "server-b",
			"conn-1": "server-a",
			" ":      "server-c",
			"conn-3": " ",
		},
	}
	store, err := NewUserRouteStoreWithRedis(client, time.Minute, "server-x", nil)
	if err != nil {
		t.Fatalf("NewUserRouteStoreWithRedis returned error: %v", err)
	}

	got, err := store.ListUserConnections(context.Background(), "user-1")
	if err != nil {
		t.Fatalf("ListUserConnections returned error: %v", err)
	}
	want := []UserConnectionRoute{
		{ConnectionID: "conn-1", ServerID: "server-a"},
		{ConnectionID: "conn-2", ServerID: "server-b"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ListUserConnections = %v, want %v", got, want)
	}
}

func TestUserRouteStorePollExpiredUsers(t *testing.T) {
	client := &fakeUserRouteRedis{
		expiredUserIDs: []string{"user-1", " ", "user-2"},
	}
	store, err := NewUserRouteStoreWithRedis(client, time.Minute, "server-x", nil)
	if err != nil {
		t.Fatalf("NewUserRouteStoreWithRedis returned error: %v", err)
	}

	var got [][]string
	err = store.PollExpiredUsers(context.Background(), 3, 20, time.UnixMilli(9_999), func(_ context.Context, userIDs []string) error {
		got = append(got, append([]string(nil), userIDs...))
		return nil
	})
	if err != nil {
		t.Fatalf("PollExpiredUsers returned error: %v", err)
	}
	if client.pollKey != userExpireKey(3) {
		t.Fatalf("poll key = %q, want %q", client.pollKey, userExpireKey(3))
	}
	if client.pollMax != 9_999 {
		t.Fatalf("poll max = %d, want %d", client.pollMax, 9_999)
	}
	if client.pollLimit != 20 {
		t.Fatalf("poll limit = %d, want %d", client.pollLimit, 20)
	}
	want := [][]string{{"user-1", "user-2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("callback users = %v, want %v", got, want)
	}

	got = nil
	err = store.PollExpiredUsers(context.Background(), 3, 20, time.UnixMilli(9_999), func(_ context.Context, userIDs []string) error {
		got = append(got, append([]string(nil), userIDs...))
		return nil
	})
	if err != nil {
		t.Fatalf("second PollExpiredUsers returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("second callback users = %v, want none", got)
	}
}

func TestUserRouteStorePropagatesCallbackError(t *testing.T) {
	client := &fakeUserRouteRedis{
		expiredUserIDs: []string{"user-1"},
	}
	store, err := NewUserRouteStoreWithRedis(client, time.Minute, "server-x", nil)
	if err != nil {
		t.Fatalf("NewUserRouteStoreWithRedis returned error: %v", err)
	}

	wantErr := errors.New("stop")
	err = store.PollExpiredUsers(context.Background(), 0, 1, time.Now(), func(context.Context, []string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("PollExpiredUsers error = %v, want %v", err, wantErr)
	}
}

func TestBucketAndKeysUseHashTag(t *testing.T) {
	bucket := bucketOf("user-1")
	if bucket < 0 || bucket >= userRouteBucketCount {
		t.Fatalf("bucket = %d out of range", bucket)
	}
	key := userRouteKey(bucket, "user-1")
	expireKey := userExpireKey(bucket)
	if !containsHashTag(key, bucket) {
		t.Fatalf("user route key %q missing bucket hash tag", key)
	}
	if !containsHashTag(expireKey, bucket) {
		t.Fatalf("user expire key %q missing bucket hash tag", expireKey)
	}
}

func containsHashTag(key string, bucket int) bool {
	return strings.Contains(key, "{"+strconv.Itoa(bucket)+"}")
}

type fakeUserRouteRedis struct {
	runScript *redis.Script
	runKeys   []string
	runArgs   []interface{}
	runErr    error

	hashValues map[string]string
	hashErr    error

	expiredUserIDs []string
	pollKey        string
	pollMax        int64
	pollLimit      int
	pollErr        error
}

func (f *fakeUserRouteRedis) RunScript(_ context.Context, script *redis.Script, keys []string, args ...interface{}) error {
	f.runScript = script
	f.runKeys = append([]string(nil), keys...)
	f.runArgs = append([]interface{}(nil), args...)
	return f.runErr
}

func (f *fakeUserRouteRedis) HGetAll(context.Context, string) (map[string]string, error) {
	return f.hashValues, f.hashErr
}

func (f *fakeUserRouteRedis) PollExpiredUsers(_ context.Context, key string, max int64, limit int) ([]string, error) {
	f.pollKey = key
	f.pollMax = max
	f.pollLimit = limit
	if f.pollErr != nil {
		return nil, f.pollErr
	}
	out := append([]string(nil), f.expiredUserIDs...)
	f.expiredUserIDs = nil
	return out, nil
}
