package commit

import (
	"context"
	"testing"
	"time"

	"go.vallahaye.net/batcher"
)

func TestThrottle(t *testing.T) {
	for _, params := range []struct {
		name      string
		commitFn  batcher.CommitFunc[int, int]
		interval  time.Duration
		mustPanic bool
	}{
		{
			name:      "nil commit func",
			commitFn:  nil,
			mustPanic: true,
		},
		{
			name:      "negative interval",
			commitFn:  func(_ context.Context, _ batcher.Operations[int, int]) {},
			interval:  -1 * time.Second,
			mustPanic: true,
		},
		{
			name:     "interval equals 1s",
			commitFn: func(_ context.Context, _ batcher.Operations[int, int]) {},
			interval: 1 * time.Second,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			const (
				size = 3
				dt   = 100 * time.Millisecond
			)

			defer func() {
				switch r := recover(); {
				case params.mustPanic && r == nil:
					t.Error("expected panic")
				case !params.mustPanic && r != nil:
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			commitFn := Throttle(params.commitFn, params.interval)

			for i, interval := range [size]time.Duration{0, params.interval, 0} {
				if i == size-1 {
					// Cancel the context to check that the commit function is called immediately.
					cancel()
				}

				committedAt := time.Now()
				commitFn(ctx, nil)

				if elapsed := time.Since(committedAt); elapsed-dt > interval {
					t.Errorf("unexpected interval: got %s, want %s⩲%s", elapsed, interval, dt)
				}
			}
		})
	}
}
