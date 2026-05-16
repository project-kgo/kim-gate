package app

import (
	"bytes"
	"context"
	"log/slog"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/project-kgo/kim-gate/internal/config"
)

func TestPollExpiredUserRoutesOnceScansAllBucketsAndLogsBatches(t *testing.T) {
	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, nil))
	poller := &fakeExpiredUserPoller{
		usersByBucket: map[int][]string{
			0: {"user-1", "user-2"},
			3: {"user-3"},
		},
	}
	app := &App{
		cfg:        config.Config{RedisRouteTTL: time.Minute},
		logger:     logger,
		userRoutes: poller,
	}
	now := time.UnixMilli(12345)

	app.pollExpiredUserRoutesOnce(context.Background(), now)

	if len(poller.calls) != 16 {
		t.Fatalf("poll calls = %d, want 16", len(poller.calls))
	}
	for bucket, call := range poller.calls {
		if call.bucket != bucket {
			t.Fatalf("call[%d].bucket = %d, want %d", bucket, call.bucket, bucket)
		}
		if call.limit != userRoutePollLimit {
			t.Fatalf("call[%d].limit = %d, want %d", bucket, call.limit, userRoutePollLimit)
		}
		if !call.now.Equal(now) {
			t.Fatalf("call[%d].now = %s, want %s", bucket, call.now, now)
		}
	}
	if !reflect.DeepEqual(poller.callbackBatches, [][]string{{"user-1", "user-2"}, {"user-3"}}) {
		t.Fatalf("callback batches = %v", poller.callbackBatches)
	}
	logOutput := logBuf.String()
	if !strings.Contains(logOutput, "expired user routes polled") {
		t.Fatalf("log output = %q", logOutput)
	}
	if !strings.Contains(logOutput, "user-1") || !strings.Contains(logOutput, "user-3") {
		t.Fatalf("log output missing user ids: %q", logOutput)
	}
}

type fakeExpiredUserPoller struct {
	usersByBucket   map[int][]string
	errByBucket     map[int]error
	calls           []fakePollCall
	callbackBatches [][]string
}

type fakePollCall struct {
	bucket int
	limit  int
	now    time.Time
}

func (f *fakeExpiredUserPoller) PollExpiredUsers(ctx context.Context, bucket int, limit int, now time.Time, fn func(context.Context, []string) error) error {
	f.calls = append(f.calls, fakePollCall{
		bucket: bucket,
		limit:  limit,
		now:    now,
	})
	if err := f.errByBucket[bucket]; err != nil {
		return err
	}
	userIDs := append([]string(nil), f.usersByBucket[bucket]...)
	if len(userIDs) == 0 {
		return nil
	}
	f.callbackBatches = append(f.callbackBatches, userIDs)
	return fn(ctx, userIDs)
}
