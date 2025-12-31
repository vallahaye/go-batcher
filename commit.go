package batcher

import (
	"context"
)

// A commit function processes a non-empty batch of operations each time it is
// invoked.
//
// For every operation in the batch, the commit function must report either a
// result or an error to any waiting consumers. This can be done per operation
// using [Operation.SignalResult] and [Operation.SignalError], or for the
// entire batch using [Operations.SignalError].
//
// The commit function must recover from any panics that occur while processing
// the batch.
//
// The commit function may be invoked with an already expired context (e.g.,
// when latent operations are committed after the batching process is
// interrupted).
//
// The memory backing the [Operations] slice is reused after the commit
// function returns. Therefore, the commit function must copy the slice if it
// retains the batch after returning (e.g., for background processing).
//
// The [commit] package contains reusable middlewares for commit functions.
type CommitFunc[T, R any] func(ctx context.Context, ops Operations[T, R])
