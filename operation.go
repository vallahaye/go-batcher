package batcher

import (
	"context"
)

type Operation[T, R any] struct {
	Value  T
	result R
	err    error
	done   chan struct{}
}

func newOperation[T, R any](v T) *Operation[T, R] {
	return &Operation[T, R]{
		Value: v,
		done:  make(chan struct{}),
	}
}

// SignalResult completes the operation with the given result and notifies any
// waiting consumers. It panics if the operation has already completed.
func (o *Operation[T, R]) SignalResult(result R) {
	o.result = result
	close(o.done)
}

// SignalError completes the operation with the given error and notifies any
// waiting consumers. It panics if the operation has already completed.
func (o *Operation[T, R]) SignalError(err error) {
	o.err = err
	close(o.done)
}

// Wait blocks until the operation completes, returning the result or the error
// encountered. If the provided context expires before the operation is
// complete, Wait returns the context's error.
func (o *Operation[T, R]) Wait(ctx context.Context) (R, error) {
	var zero R
	select {
	case <-o.done:
		if o.err != nil {
			return zero, o.err
		}
		return o.result, nil
	case <-ctx.Done():
		return zero, ctx.Err()
	}
}
