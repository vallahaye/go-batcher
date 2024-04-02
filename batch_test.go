package batcher

import (
	"context"
	"testing"
	"time"
)

func TestBatch(t *testing.T) {
	for _, params := range []struct {
		name    string
		maxSize int
		timeout time.Duration
	}{
		{
			name:    "max size equals 10 and no timeout",
			maxSize: 10,
			timeout: NoTimeout,
		},
		{
			name:    "unlimited size and timeout equals 1s",
			maxSize: UnlimitedSize,
			timeout: 1 * time.Second,
		},
		{
			name:    "max size equals 10 and timeout equals 1s",
			maxSize: 10,
			timeout: 1 * time.Second,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			totalSize := max(2*params.maxSize, 10)
			longTimeout := max(2*params.timeout, 1*time.Second)

			in := make(chan *Operation[int, int])
			c := make(chan time.Time, totalSize)

			go func() {
				defer cancel()

				ops := make([]*Operation[int, int], totalSize)
				for i := 0; i < len(ops); i++ {
					if i == 0 || i == len(ops)-1 || (params.maxSize != UnlimitedSize && i%params.maxSize == 0) {
						time.Sleep(longTimeout)
					}
					ops[i] = newOperation[int, int](i)
					in <- ops[i]
					c <- time.Now()
				}

				for _, op := range ops {
					if _, err := op.Wait(ctx); err != nil {
						t.Errorf("unexpected wait error: %v", err)
					}
				}
			}()

			commitFn := func(_ context.Context, out []*Operation[int, int]) {
				const dt = 100 * time.Millisecond

				elapsed := time.Since(<-c)
				for i := 1; i < len(out); i++ {
					<-c
				}

				t.Logf("committed batch: len(out) = %d, elapsed = %s", len(out), elapsed)

				switch {
				case len(out) == 0:
					t.Error("unexpected batch size: got 0")
				case params.maxSize != UnlimitedSize && len(out) > params.maxSize:
					t.Errorf("unexpected batch size: got %d, want at most %d", len(out), params.maxSize)
				case params.timeout != NoTimeout && elapsed-dt > params.timeout:
					t.Errorf("unexpected timeout: got %s, want at most %s⩲%s", elapsed, params.timeout, dt)
				}

				for _, op := range out {
					op.SetResult(op.Value)
				}
			}

			batch(ctx, in, params.maxSize, params.timeout, commitFn)
		})
	}
}
