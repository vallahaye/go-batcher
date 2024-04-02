package batcher

import "context"

type CommitFunc[T, R any] func(context.Context, []*Operation[T, R])
