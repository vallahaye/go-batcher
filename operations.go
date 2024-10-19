package batcher

type Operations[T, R any] []*Operation[T, R]

// SetError signals an error to all operations.
func (o Operations[T, R]) SetError(err error) {
	for _, op := range o {
		op.SetError(err)
	}
}
