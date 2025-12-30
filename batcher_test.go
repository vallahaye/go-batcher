package batcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestNewBatcher(t *testing.T) {
	for _, params := range []struct {
		name      string
		commitFn  CommitFunc[int, int]
		opts      []Option[int, int]
		maxSize   int
		timeout   time.Duration
		mustPanic bool
	}{
		{
			name:      "nil commit func",
			commitFn:  nil,
			mustPanic: true,
		},
		{
			name:     "negative max size",
			commitFn: func(_ context.Context, _ Operations[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](-1),
			},
			mustPanic: true,
		},
		{
			name:     "negative timeout",
			commitFn: func(_ context.Context, _ Operations[int, int]) {},
			opts: []Option[int, int]{
				WithTimeout[int, int](-1 * time.Second),
			},
			mustPanic: true,
		},
		{
			name:     "unlimited size with no timeout",
			commitFn: func(_ context.Context, _ Operations[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](UnlimitedSize),
				WithTimeout[int, int](NoTimeout),
			},
			mustPanic: true,
		},
		{
			name:      "unlimited size with no timeout (no option provided)",
			commitFn:  func(_ context.Context, _ Operations[int, int]) {},
			opts:      nil,
			mustPanic: true,
		},
		{
			name:     "max size equals 10",
			commitFn: func(_ context.Context, _ Operations[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](10),
			},
			maxSize: 10,
			timeout: NoTimeout,
		},
		{
			name:     "timeout equals 1s",
			commitFn: func(_ context.Context, _ Operations[int, int]) {},
			opts: []Option[int, int]{
				WithTimeout[int, int](1 * time.Second),
			},
			maxSize: UnlimitedSize,
			timeout: 1 * time.Second,
		},
		{
			name:     "max size equals 10 and timeout equals 1s",
			commitFn: func(_ context.Context, _ Operations[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](10),
				WithTimeout[int, int](1 * time.Second),
			},
			maxSize: 10,
			timeout: 1 * time.Second,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			var b *Batcher[int, int]
			defer func() {
				switch r := recover(); {
				case params.mustPanic && r == nil:
					t.Error("expected panic")
				case !params.mustPanic && r != nil:
					t.Errorf("unexpected panic: %v", r)
				case !params.mustPanic && r == nil:
					if b.maxSize != params.maxSize {
						t.Errorf("unexpected max size: got %d, want %d", b.maxSize, params.maxSize)
					}
					if b.timeout != params.timeout {
						t.Errorf("unexpected timeout: got %s, want %s", b.timeout, params.timeout)
					}
				}
			}()

			b = New(params.commitFn, params.opts...)
		})
	}
}

func TestBatcherSend(t *testing.T) {
	for _, params := range []struct {
		name  string
		value int
		err   error
	}{
		{
			name:  "send value",
			value: 1,
		},
		{
			name: "send expires error",
			err:  context.DeadlineExceeded,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()

			b := New(func(_ context.Context, _ Operations[int, int]) {}, WithMaxSize[int, int](1))

			var wg sync.WaitGroup
			if params.err == nil || !errors.Is(params.err, context.DeadlineExceeded) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					b.Batch(ctx)
				}()
			}

			op, err := b.Send(ctx, params.value)

			switch {
			case err == nil && params.err != nil:
				t.Error("expected error")
			case err != nil && params.err == nil:
				t.Errorf("unexpected error: %v", err)
			case err != nil && !errors.Is(err, params.err):
				t.Errorf("unexpected error: got %v, want %v", err, params.err)
			case err == nil && op.Value != params.value:
				t.Errorf("unexpected value: got %d, want %d", op.Value, params.value)
			}

			wg.Wait()
		})
	}
}

func TestBatcherBatch(t *testing.T) {
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
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			totalSizeCommitted := 0
			commitFn := func(_ context.Context, ops Operations[time.Time, time.Time]) {
				const dt = 100 * time.Millisecond

				if len(ops) == 0 {
					t.Error("empty batch committed")
					return
				}

				elapsed := time.Since(ops[0].Value)
				t.Logf("committed batch: len(ops) = %d, elapsed = %s", len(ops), elapsed)

				switch {
				case params.maxSize != UnlimitedSize && len(ops) > params.maxSize:
					t.Errorf("unexpected batch size: got %d, want at most %d", len(ops), params.maxSize)
				case params.timeout != NoTimeout && elapsed-dt > params.timeout:
					t.Errorf("unexpected timeout: got %s, want at most %s⩲%s", elapsed, params.timeout, dt)
				}

				totalSizeCommitted += len(ops)
			}
			b := New(commitFn, WithMaxSize[time.Time, time.Time](params.maxSize), WithTimeout[time.Time, time.Time](params.timeout))

			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				defer wg.Done()
				b.Batch(ctx)
			}()

			totalSize := max(2*params.maxSize, 10)
			greaterTimeout := params.timeout + 1*time.Second
			for i := range totalSize {
				switch i {
				case 0:
					// Simulate a delay to check that the batcher doesn't timeout while receiving the first operation.
					time.Sleep(greaterTimeout)

				case 1:
					// Simulate a delay to check that the batcher commits after a timeout.
					time.Sleep(greaterTimeout)
				}

				if _, err := b.Send(ctx, time.Now()); err != nil {
					t.Errorf("unexpected send error: %v", err)
				}
			}

			// Cancel the context to check that the batcher commits latent operations.
			cancel()
			wg.Wait()

			if totalSizeCommitted != totalSize {
				t.Errorf("unexpected total size: got %d, want %d", totalSizeCommitted, totalSize)
			}
		})
	}
}
