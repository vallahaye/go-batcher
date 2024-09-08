package commit

import (
	"context"
	"time"

	"go.vallahaye.net/batcher"
)

// Timeout calls the commit function with a timeout set to the context. It
// panics if the commit function is nil.
func Timeout[T, R any](commitFn batcher.CommitFunc[T, R], timeout time.Duration) batcher.CommitFunc[T, R] {
	if commitFn == nil {
		panic("batcher: nil commit func")
	}

	return func(parent context.Context, ops []*batcher.Operation[T, R]) {
		ctx, cancel := context.WithTimeout(parent, timeout)
		defer cancel()

		commitFn(ctx, ops)
	}
}
