package batcher

type Operations[T, R any] []*Operation[T, R]

// SignalError completes all operations with the given error and notifies any
// waiting consumers. It panics if any operation has already completed.
func (o Operations[T, R]) SignalError(err error) {
	for _, op := range o {
		op.SignalError(err)
	}
}
