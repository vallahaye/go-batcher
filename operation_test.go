package batcher

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewOperation(t *testing.T) {
	const v = 1
	op := newOperation[int, int](v)
	if op.Value != v {
		t.Errorf("expected value: got %d, want %d", op.Value, v)
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
			err:  errors.New("wait error"),
		},
		{
			name: "wait expires error",
			err:  context.DeadlineExceeded,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			op := newOperation[int, int](0)
			switch {
			case params.err == nil:
				op.SetResult(params.result)
			case !errors.Is(params.err, context.DeadlineExceeded):
				op.SetError(params.err)
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
