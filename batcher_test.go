package batcher

import (
	"context"
	"errors"
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
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](-1),
			},
			mustPanic: true,
		},
		{
			name:     "negative timeout",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			opts: []Option[int, int]{
				WithTimeout[int, int](-1 * time.Second),
			},
			mustPanic: true,
		},
		{
			name:     "unlimited size with no timeout",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](UnlimitedSize),
				WithTimeout[int, int](NoTimeout),
			},
			mustPanic: true,
		},
		{
			name:      "unlimited size with no timeout (no option provided)",
			commitFn:  func(_ context.Context, _ []*Operation[int, int]) {},
			opts:      nil,
			mustPanic: true,
		},
		{
			name:     "max size equals 10",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			opts: []Option[int, int]{
				WithMaxSize[int, int](10),
			},
			maxSize: 10,
			timeout: NoTimeout,
		},
		{
			name:     "timeout equals 1s",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			opts: []Option[int, int]{
				WithTimeout[int, int](1 * time.Second),
			},
			maxSize: UnlimitedSize,
			timeout: 1 * time.Second,
		},
		{
			name:     "max size equals 10 and timeout equals 1s",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
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
				v := recover()
				switch {
				case params.mustPanic && v == nil:
					t.Errorf("expected panic")
				case !params.mustPanic && v != nil:
					t.Errorf("unexpected panic: %v", v)
				case !params.mustPanic && v == nil:
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

func TestBatcherAdd(t *testing.T) {
	for _, params := range []struct {
		name string
		err  error
	}{
		{
			name: "add",
		},
		{
			name: "add expires error",
			err:  context.DeadlineExceeded,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			b := &Batcher[int, int]{
				in: make(chan *Operation[int, int]),
			}
			if params.err == nil || !errors.Is(params.err, context.DeadlineExceeded) {
				go func() {
					<-b.in
				}()
			}

			_, err := b.Add(ctx, 1)

			switch {
			case err == nil && params.err != nil:
				t.Error("expected error")
			case err != nil && !errors.Is(err, params.err):
				t.Errorf("unexpected error: got %v, want %v", err, params.err)
			}
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
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			totalSize := 0

			b := &Batcher[time.Time, time.Time]{
				commitFn: func(_ context.Context, out []*Operation[time.Time, time.Time]) {
					const dt = 100 * time.Millisecond

					elapsed := time.Since(out[0].Value)
					t.Logf("committed batch: len(out) = %d, elapsed = %s", len(out), elapsed)

					switch {
					case params.maxSize != UnlimitedSize && len(out) > params.maxSize:
						t.Errorf("unexpected batch size: got %d, want at most %d", len(out), params.maxSize)
					case params.timeout != NoTimeout && elapsed-dt > params.timeout:
						t.Errorf("unexpected timeout: got %s, want at most %s⩲%s", elapsed, params.timeout, dt)
					}

					totalSize += len(out)
				},
				maxSize: params.maxSize,
				timeout: params.timeout,
				in:      make(chan *Operation[time.Time, time.Time]),
			}

			done := make(chan struct{})
			go func() {
				b.Batch(ctx)
				close(done)
			}()

			maxSize := max(params.maxSize, 10)
			timeout := max(2*params.timeout, 1*time.Second)

			for i := 0; i < maxSize; i++ {
				// Simulate a delay to check that the batcher is indeed waiting
				// indefinitely for the arrival of the first operation of a batch,
				// and that it commits after a timeout.
				if i == 0 || i == 1 {
					time.Sleep(timeout)
				}

				b.in <- &Operation[time.Time, time.Time]{
					Value: time.Now(),
				}
			}

			// Cancel the context to check that the batcher commits latent
			// operations.
			cancel()
			<-done

			if totalSize != maxSize {
				t.Errorf("unexpected total size: got %d, want %d", totalSize, maxSize)
			}
		})
	}
}
