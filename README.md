# Go Batcher

[![Go Reference](https://pkg.go.dev/badge/go.vallahaye.net/batcher.svg)](https://pkg.go.dev/go.vallahaye.net/batcher)
[![Go Report Card](https://goreportcard.com/badge/go.vallahaye.net/batcher)](https://goreportcard.com/report/go.vallahaye.net/batcher)

Go Batcher provides a generic and versatile implementation of a batching algorithm for Golang, with no third-party dependencies. The algorithm can be constrained in [space](https://pkg.go.dev/go.vallahaye.net/batcher#WithMaxSize) and [time](https://pkg.go.dev/go.vallahaye.net/batcher#WithTimeout), with a simple yet robust API, enabling developers to easily incorporate batching into their live services.

### Goals

- Easy to use and minimal API surface area
- Efficient use of resources, i.e. reuse of memory and good use of goroutines parking behavior
- Well-tested and documented

### Non-goals

- Provide helper functions to interface Go Batcher with other software solutions (for example, connecting Go Batcher to a distributed task/job framework)

## Example

```go
// Define a commit function.
commitFn := func(ctx context.Context, ops batcher.Operations[string, string]) {
  // Watch the context's Done channel to know when the batching process gets interrupted.
  //
  // Do something with the batch of operations.
  // See [Operations.SetError] to signal an error to all operations.
  for _, op := range ops {
    // See [Operation.SetResult] and [Operation.SetError] to signal individual results and errors.
  }
}

// Create a batcher committing a batch every 10 operations.
b := batcher.New(commitFn, batcher.WithMaxSize[string, string](10))

// Run the batcher in the background. Cancel the context to interrupt the batching process.
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

go func() {
  b.Batch(ctx)
}()

http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
  // Send a value to the batcher.
  op, err := b.Send(r.Context(), r.FormValue("value"))
  if err != nil {
    // Do something with the error.
  }

  // Get the operation's result.
  result, err := op.Wait(r.Context())
  if err != nil {
    // Do something with the error.
  }

  // Do something with the operation's result.
})
```
