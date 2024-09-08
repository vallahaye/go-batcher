package commit

import (
	"context"
	"time"

	"go.vallahaye.net/batcher"
)

// Throttle calls the commit function at most once per time interval. It panics
// if the commit function is nil or the interval is negative. The commit
// function is called immediately if the context expires while waiting.
func Throttle[T, R any](commitFn batcher.CommitFunc[T, R], interval time.Duration) batcher.CommitFunc[T, R] {
	if commitFn == nil {
		panic("batcher: nil commit func")
	}

	if interval < 0 {
		panic("batcher: negative commit throttle interval")
	}

	t := time.NewTimer(0)
	return func(ctx context.Context, ops []*batcher.Operation[T, R]) {
		defer t.Reset(interval)

		select {
		case <-t.C:
		case <-ctx.Done():
		}

		commitFn(ctx, ops)
	}
}
