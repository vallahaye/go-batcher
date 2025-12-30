package commit

import (
	"context"
	"testing"
	"time"

	"go.vallahaye.net/batcher"
)

func TestTimeout(t *testing.T) {
	for _, params := range []struct {
		name      string
		commitFn  batcher.CommitFunc[int, int]
		timeout   time.Duration
		mustPanic bool
	}{
		{
			name:      "nil commit func",
			commitFn:  nil,
			mustPanic: true,
		},
		{
			name: "timeout equals 1s",
			commitFn: func(ctx context.Context, _ batcher.Operations[int, int]) {
				<-ctx.Done()
			},
			timeout: 1 * time.Second,
		},
	} {
		t.Run(params.name, func(t *testing.T) {
			const dt = 100 * time.Millisecond

			defer func() {
				switch r := recover(); {
				case params.mustPanic && r == nil:
					t.Error("expected panic")
				case !params.mustPanic && r != nil:
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			commitFn := Timeout(params.commitFn, params.timeout)

			committedAt := time.Now()
			commitFn(t.Context(), nil)

			if elapsed := time.Since(committedAt); elapsed-dt > params.timeout {
				t.Errorf("unexpected timeout: got %s, want %s⩲%s", elapsed, params.timeout, dt)
			}
		})
	}
}
