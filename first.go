package main

import (
	"context"
	"errors"
	"sync"

	"golang.org/x/sync/errgroup"
)

// FirstResponse is a helper to get the first non-null result or error from a set of goroutines.
type FirstResponse struct {
	result     chan any
	wg         *errgroup.Group
	waitWg     chan struct{}
	resultOnce sync.Once
	ctx        context.Context
}

func NewFirstResponse(ctx context.Context, concurrency int) *FirstResponse {
	fr := &FirstResponse{
		result: make(chan any),
		waitWg: make(chan struct{}),
	}
	fr.wg, ctx = errgroup.WithContext(ctx)
	if concurrency > 0 {
		fr.wg.SetLimit(concurrency)
	}
	fr.ctx = ctx
	return fr
}

// Spawn spawns a goroutine that executes the given function.
func (w *FirstResponse) Spawn(f func() (any, error)) {
	w.wg.Go(func() error {
		result, err := f()
		if err != nil {
			w.send(err)
			return errGotFirstResult // stop the errgroup
		} else {
			if result != nil {
				w.send(result)
				return errGotFirstResult // stop the errgroup
			}
		}
		return nil
	})
}

var errGotFirstResult = errors.New("got first result")

// send sends the result to the channel, but only once.
// If the result is already sent, it does nothing.
// The result can be something, or an error.
func (w *FirstResponse) send(result any) {
	w.resultOnce.Do(func() {
		w.result <- result
		close(w.result)
	})
}

// Wait waits for all goroutines to finish, and returns the first non-null result or error.
func (w *FirstResponse) Wait() any {
	go func() {
		w.wg.Wait()
		w.waitWg <- struct{}{}
	}()

	select {
	case result := <-w.result:
		return result
	case <-w.waitWg:
		return nil
	}
}
