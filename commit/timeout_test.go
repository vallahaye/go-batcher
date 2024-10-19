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
				v := recover()
				switch {
				case params.mustPanic && v == nil:
					t.Errorf("expected panic")
				case !params.mustPanic && v != nil:
					t.Errorf("unexpected panic: %v", v)
				}
			}()

			commitFn := Timeout(params.commitFn, params.timeout)

			committedAt := time.Now()
			commitFn(context.Background(), nil)

			if elapsed := time.Since(committedAt); elapsed-dt > params.timeout {
				t.Errorf("unexpected timeout: got %s, want %s⩲%s", elapsed, params.timeout, dt)
			}
		})
	}
}
