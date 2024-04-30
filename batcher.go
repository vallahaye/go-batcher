package batcher

import (
	"context"
	"time"
)

// Batch an unlimited amount of operations.
const UnlimitedSize = 0

// Batch operations for an infinite duration.
const NoTimeout time.Duration = 0

type Option[T, R any] func(*Batcher[T, R])

// WithMaxSize configures the max size constraint on a batcher.
func WithMaxSize[T, R any](maxSize int) Option[T, R] {
	return func(b *Batcher[T, R]) {
		b.maxSize = maxSize
	}
}

// WithTimeout configures the timeout constraint on a batcher.
func WithTimeout[T, R any](timeout time.Duration) Option[T, R] {
	return func(b *Batcher[T, R]) {
		b.timeout = timeout
	}
}

type Batcher[T, R any] struct {
	commitFn CommitFunc[T, R]
	maxSize  int
	timeout  time.Duration
	in       chan *Operation[T, R]
}

// New creates a new batcher, calling the commit function each time it
// completes a batch of operations according to its options. It panics if the
// commit function is nil, max size is negative, timeout is negative or max
// size equals [UnlimitedSize] and timeout equals [NoTimeout] (the default if
// no options are provided).
//
// Some examples:
//
// Create a batcher committing a batch every 10 operations:
//
//	New[T, R](commitFn, WithMaxSize(10))
//
// Create a batcher committing a batch 1 second after accepting the first
// operation:
//
//	New[T, R](commitFn, WithTimeout(1 * time.Second))
//
// Create a batcher committing a batch containing at most 10 operations and at
// most 1 second after accepting the first operation:
//
//	New[T, R](commitFn, WithMaxSize(10), WithTimeout(1 * time.Second))
func New[T, R any](commitFn CommitFunc[T, R], opts ...Option[T, R]) *Batcher[T, R] {
	b := &Batcher[T, R]{
		commitFn: commitFn,
		maxSize:  UnlimitedSize,
		timeout:  NoTimeout,
		in:       make(chan *Operation[T, R]),
	}

	for _, opt := range opts {
		opt(b)
	}

	if b.commitFn == nil {
		panic("batcher: nil commit func")
	}

	if b.maxSize < 0 {
		panic("batcher: negative max size")
	}

	if b.timeout < 0 {
		panic("batcher: negative timeout")
	}

	if b.maxSize == UnlimitedSize && b.timeout == NoTimeout {
		panic("batcher: unlimited size with no timeout")
	}

	return b
}

// Add creates a new operation and sends it to the batcher in a blocking
// fashion. If the provided context expires before the batcher accepts the
// operation, Add returns the context's error.
func (b *Batcher[T, R]) Add(ctx context.Context, v T) (*Operation[T, R], error) {
	op := newOperation[T, R](v)
	select {
	case b.in <- op:
		return op, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Batch receives operations from the batcher, calling the commit function
// whenever max size is reached or a timeout occurs. It waits indefinitely for
// the first operation of each batch to arrive.
//
// When the provided context expires, the batching process is interrupted and
// the function returns after a final call to the commit function. The latter
// is ignored if there are no latent operations.
func (b *Batcher[T, R]) Batch(ctx context.Context) {
	var out []*Operation[T, R]
	if b.maxSize != UnlimitedSize {
		out = make([]*Operation[T, R], 0, b.maxSize)
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
			if len(out) == b.maxSize {
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

			// We reset the timer channel to wait indefinitely for the first
			// operation of the next batch to arrive. A nil channel is never selected
			// for reading.
			c = nil
			// We reset the slice while preserving the allocated memory.
			out = out[:0]
		}

		if done {
			break
		}

		if !commit && c == nil && b.timeout != NoTimeout {
			if t == nil {
				t = time.NewTimer(b.timeout)
			} else {
				if !t.Stop() {
					select {
					case <-t.C:
					default:
					}
				}
				t.Reset(b.timeout)
			}
			c = t.C
		}
	}
}
