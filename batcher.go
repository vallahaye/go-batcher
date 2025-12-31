package batcher

import (
	"context"
	"time"
)

// Batch an unlimited amount of operations.
const UnlimitedSize = 0

// Batch operations indefinitely.
const NoTimeout time.Duration = 0

type Batcher[T, R any] struct {
	commitFn CommitFunc[T, R]
	size     int
	timeout  time.Duration
	in       chan *Operation[T, R]
}

// New creates a new batcher, with a commit function, a size and a timeout
// constraint. It panics if the commit function is nil, size is negative,
// timeout is negative, or size equals [UnlimitedSize] and timeout equals
// [NoTimeout].
//
// Some examples:
//
// Create a batcher committing a batch every 10 operations:
//
//	New[T, R](commitFn, 10, NoTimeout)
//
// Create a batcher committing a batch every second:
//
//	New[T, R](commitFn, UnlimitedSize, 1*time.Second)
//
// Create a batcher committing a batch every 10 operations or every second:
//
//	New[T, R](commitFn, 10, 1*time.Second)
//
// See also:
//
//   - [CommitFunc] for more information about commit functions
//   - [Batcher.Batch] to start the batching process
//   - [Batcher.Send] to create and send an operation to the batcher
func New[T, R any](commitFn CommitFunc[T, R], size int, timeout time.Duration) *Batcher[T, R] {
	if commitFn == nil {
		panic("batcher: nil commit func")
	}

	if size < 0 {
		panic("batcher: negative size")
	}

	if timeout < 0 {
		panic("batcher: negative timeout")
	}

	if size == UnlimitedSize && timeout == NoTimeout {
		panic("batcher: unlimited size with no timeout")
	}

	return &Batcher[T, R]{
		commitFn: commitFn,
		size:     size,
		timeout:  timeout,
		in:       make(chan *Operation[T, R]),
	}
}

// Send creates a new operation and sends it to the batcher in a blocking
// fashion. If the provided context expires before the batcher receives the
// operation, Send returns the context's error.
//
// See also:
//
//   - [Operation.Wait] to get the operation's result or error
func (b *Batcher[T, R]) Send(ctx context.Context, v T) (*Operation[T, R], error) {
	op := newOperation[T, R](v)
	select {
	case b.in <- op:
		return op, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Batch receives operations from the batcher in a blocking fashion, invoking
// the commit function whenever the size or timeout constraint is reached.
// Timeouts are disabled while receiving the first operation of each batch.
//
// When the provided context expires, the batching process is interrupted and
// the function returns. Before returning, a final call to the commit function
// is made if there are any latent operations; otherwise, it is skipped.
func (b *Batcher[T, R]) Batch(ctx context.Context) {
	var out Operations[T, R]
	if b.size != UnlimitedSize {
		out = make(Operations[T, R], 0, b.size)
	}

	var (
		t *time.Timer
		c <-chan time.Time
	)

	for {
		var commit, done bool

		select {
		case op := <-b.in:
			out = append(out, op)
			if len(out) == b.size {
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
			b.commitFn(ctx, out)
			out = out[:0]
			c = nil
		}

		if done {
			break
		}

		if !commit && c == nil && b.timeout != NoTimeout {
			if t == nil {
				t = time.NewTimer(b.timeout)
			} else {
				t.Reset(b.timeout)
			}
			c = t.C
		}
	}
}
