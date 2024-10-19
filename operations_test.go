package batcher

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestOperationsSetError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	ops := make(Operations[int, int], 2)
	for i := 0; i < len(ops); i++ {
		ops[i] = newOperation[int, int](i)
	}

	want := errors.New("operation error")
	ops.SetError(want)

	for _, op := range ops {
		switch _, got := op.Wait(ctx); {
		case got == nil:
			t.Error("expected error")
		case !errors.Is(got, want):
			t.Errorf("unexpected error: got %v, want %v", got, want)
		}
	}
}
