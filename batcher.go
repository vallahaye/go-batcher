package batcher

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

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

	in      chan *Operation[T, R]
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running atomic.Bool
}

// New creates a new batcher, calling the commit function each time it
// completes a batch of operations according to its options. It panics if the
// commit function is nil, max size is negative, timeout is negative or max
// size equals UnlimitedSize and timeout equals NoTimeout (the default if no
// options are provided).
//
// Some examples:
//
//	New[T, R](commitFn, WithMaxSize(10)) // creates a batcher committing a batch every 10 operations.
//	New[T, R](commitFn, WithTimeout(1 * time.Second)) // creates a batcher committing a batch 1 second after accepting the first operation.
//	New[T, R](commitFn, WithMaxSize(10), WithTimeout(1 * time.Second)) // creates a batcher committing a batch containing at most 10 operations and at most 1 second after accepting the first operation.
func New[T, R any](commitFn CommitFunc[T, R], opts ...Option[T, R]) *Batcher[T, R] {
	b := &Batcher[T, R]{
		commitFn: commitFn,
		maxSize:  UnlimitedSize,
		timeout:  NoTimeout,

		in: make(chan *Operation[T, R]),
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

	// Batching an unlimited number of operations for an infinite duration would
	// not allow a single batch to be completed.
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

// Run ensures the batcher is running. If not, it starts it in the background.
func (b *Batcher[T, R]) Run() {
	if b.running.Load() {
		return
	}

	b.ctx, b.cancel = context.WithCancel(context.Background())
	b.wg.Add(1)
	b.running.Store(true)

	go func() {
		defer b.running.Store(false)
		defer b.wg.Done()
		defer b.cancel()

		batch(b.ctx, b.in, b.maxSize, b.timeout, b.commitFn)
	}()
}

// Shutdown gracefully shuts down the batcher by draining it and waiting the
// last commit to complete. If the provided context expires before the shutdown
// is complete, Shutdown returns the context's error.
func (b *Batcher[T, R]) Shutdown(ctx context.Context) error {
	if !b.running.Load() {
		return nil
	}

	done := make(chan struct{})
	go func() {
		b.cancel()
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
