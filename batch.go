package batcher

import (
	"context"
	"time"
)

// Batch an unlimited amount of operations.
const UnlimitedSize = 0

// Batch operations for an infinite duration.
const NoTimeout = time.Duration(0)

// batch reads operations from in and calls commitFn with a batch (a slice) of
// operations each time maxSize is reached or a timeout occurs.
//
// It waits indefinitely for the first operation of each batch to arrive.
//
// When the provided context expires, the batching process is interrupted and
// the function returns after a last call to commitFn. The latter is ignored if
// there are no last operations to batch.
func batch[T, R any](ctx context.Context, in <-chan *Operation[T, R], maxSize int, timeout time.Duration, commitFn CommitFunc[T, R]) {
	var out []*Operation[T, R]
	if maxSize != UnlimitedSize {
		out = make([]*Operation[T, R], 0, maxSize)
	}

	var (
		t *time.Timer
		c <-chan time.Time
	)

	for {
		var commit, done bool
		select {
		case op := <-in:
			out = append(out, op)
			if len(out) == maxSize {
				commit = true
			}
		case <-c:
			commit = true
		case <-ctx.Done():
			if len(out) > 0 {
				commit = true
			}
			done = true
		}

		if commit {
			commitFn(ctx, out)

			// We reset the timer channel to wait indefinitely for the first
			// operation of the next batch to arrive. A nil channel is never
			// selected for reading.
			c = nil
			// We reset the slice while preserving the allocated memory.
			out = out[:0]
		}

		if done {
			break
		}

		if !commit && c == nil && timeout != NoTimeout {
			if t == nil {
				t = time.NewTimer(timeout)
			} else {
				if !t.Stop() {
					select {
					case <-t.C:
					default:
					}
				}
				t.Reset(timeout)
			}
			c = t.C
		}
	}
}
