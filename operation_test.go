package batcher

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewOperation(t *testing.T) {
	v := 1
	op := newOperation[int, int](v)

	if op.Value != v {
		t.Errorf("unexpected value: got %d, want %d", op.Value, v)
	}
}

func TestOperationSetResult(t *testing.T) {
	op := &Operation[int, int]{
		done: make(chan struct{}),
	}

	result := 1
	op.SetResult(result)

	if op.result != result {
		t.Errorf("unexpected result: got %d, want %d", op.result, result)
	}

	opened := true
	select {
	case _, opened = <-op.done:
	default:
	}

	if opened {
		t.Error("expected done channel to be closed")
	}
}

func TestOperationSetError(t *testing.T) {
	op := &Operation[int, int]{
		done: make(chan struct{}),
	}

	err := errors.New("operation error")
	op.SetError(err)

	switch {
	case op.err == nil:
		t.Error("expected error")
	case !errors.Is(op.err, err):
		t.Errorf("unexpected error: got %v, want %v", op.err, err)
	}

	opened := true
	select {
	case _, opened = <-op.done:
	default:
	}

	if opened {
		t.Error("expected done channel to be closed")
	}
}

func TestOperationWait(t *testing.T) {
	for _, params := range []struct {
		name   string
		result int
		err    error
	}{
		{
			name:   "wait result",
			result: 1,
		},
		{
			name: "wait error",
			err:  errors.New("operation error"),
		},
		{
			name: "wait expires error",
			err:  context.DeadlineExceeded,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			op := &Operation[int, int]{
				done: make(chan struct{}),
			}

			switch {
			case params.err == nil:
				op.result = params.result
				close(op.done)
			case !errors.Is(params.err, context.DeadlineExceeded):
				op.err = params.err
				close(op.done)
			}

			result, err := op.Wait(ctx)

			switch {
			case err == nil && params.err != nil:
				t.Error("expected error")
			case err != nil && params.err == nil:
				t.Errorf("unexpected error: %v", err)
			case err != nil && !errors.Is(err, params.err):
				t.Errorf("unexpected error: got %v, want %v", err, params.err)
			case err == nil && result != params.result:
				t.Errorf("unexpected result: got %d, want %d", result, params.result)
			}
		})
	}
}
