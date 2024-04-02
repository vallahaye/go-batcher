package batcher

import (
	"context"
	"errors"
	"sync/atomic"
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
		name  string
		setup func(context.Context, *Batcher[int, int]) error
		err   error
	}{
		{
			name: "add running",
			setup: func(_ context.Context, b *Batcher[int, int]) error {
				b.Run()
				return nil
			},
		},
		{
			name: "add expires error",
			setup: func(_ context.Context, _ *Batcher[int, int]) error {
				return nil
			},
			err: context.DeadlineExceeded,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			commitFn := func(_ context.Context, _ []*Operation[int, int]) {}

			b := New(commitFn, WithTimeout[int, int](2*time.Second))
			if err := params.setup(ctx, b); err != nil {
				t.Fatalf("unexpected setup error: %v", err)
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

func TestBatcherShutdown(t *testing.T) {
	for _, params := range []struct {
		name       string
		commitFn   CommitFunc[int, int]
		setup      func(context.Context, *Batcher[int, int]) error
		mustCommit bool
		err        error
	}{
		{
			name:     "shutdown not running",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			setup: func(_ context.Context, _ *Batcher[int, int]) error {
				return nil
			},
		},
		{
			name:     "shutdown running",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			setup: func(_ context.Context, b *Batcher[int, int]) error {
				b.Run()
				return nil
			},
		},
		{
			name:     "shutdown running must commit",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {},
			setup: func(ctx context.Context, b *Batcher[int, int]) error {
				b.Run()
				_, err := b.Add(ctx, 1)
				return err
			},
			mustCommit: true,
		},
		{
			name: "shutdown expires error",
			commitFn: func(_ context.Context, _ []*Operation[int, int]) {
				time.Sleep(2 * time.Second)
			},
			setup: func(ctx context.Context, b *Batcher[int, int]) error {
				b.Run()
				_, err := b.Add(ctx, 1)
				return err
			},
			err: context.DeadlineExceeded,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			var committed atomic.Bool
			commitFn := func(ctx context.Context, out []*Operation[int, int]) {
				params.commitFn(ctx, out)
				committed.Store(true)
			}

			b := New(commitFn, WithTimeout[int, int](2*time.Second))
			if err := params.setup(ctx, b); err != nil {
				t.Fatalf("unexpected setup error: %v", err)
			}

			err := b.Shutdown(ctx)

			switch {
			case err == nil && params.mustCommit && !committed.Load():
				t.Error("expected commit")
			case err == nil && params.err != nil:
				t.Error("expected error")
			case err != nil && !errors.Is(err, params.err):
				t.Errorf("unexpected error: got %v, want %v", err, params.err)
			}
		})
	}
}
