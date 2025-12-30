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

func TestOperationSignalResult(t *testing.T) {
	op := newOperation[int, int](1)

	result := 1
	op.SignalResult(result)

	if op.result != result {
		t.Errorf("unexpected result: got %d, want %d", op.result, result)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	op.SignalResult(2)
}

func TestOperationSignalError(t *testing.T) {
	op := newOperation[int, int](1)

	err := errors.New("operation error")
	op.SignalError(err)

	switch {
	case op.err == nil:
		t.Error("expected error")
	case !errors.Is(op.err, err):
		t.Errorf("unexpected error: got %v, want %v", op.err, err)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic")
		}
	}()

	op.SignalError(errors.New("another operation error"))
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
			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			op := newOperation[int, int](1)

			switch {
			case params.err == nil:
				op.SignalResult(params.result)
			case !errors.Is(params.err, context.DeadlineExceeded):
				op.SignalError(params.err)
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
