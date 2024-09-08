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
			commitFn:  func(_ context.Context, _ []*batcher.Operation[int, int]) {},
			interval:  -1 * time.Second,
			mustPanic: true,
		},
		{
			name:     "interval equals 1s",
			commitFn: func(_ context.Context, _ []*batcher.Operation[int, int]) {},
			interval: 1 * time.Second,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			const dt = 100 * time.Millisecond

			defer func() {
				v := recover()
				switch {
				case params.mustPanic && v == nil:
					t.Errorf("expected panic")
				case !params.mustPanic && v != nil:
					t.Errorf("unexpected panic: %v", v)
				}
			}()

			commitFn := Throttle(params.commitFn, params.interval)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			const size = 3
			for i, interval := range [size]time.Duration{0, params.interval, 0} {
				if i == size-1 {
					// Cancel the context to check that the commit function is called
					// immediately.
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
