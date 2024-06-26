package main

import (
	"context"
	"strconv"
	"strings"

	"golang.org/x/sync/errgroup"
)

// FirstSuccess is a helper for running multiple functions concurrently and returning the first successful result.
// If all functions return an error, all the errors are returned as a ErrorSlice.
func FirstSuccess[T comparable](
	ctx context.Context,
	concurrency int,
	fns ...JobFunc[T],
) (T, error) {
	type result struct {
		val T
		err error
	}
	results := make(chan result, len(fns))
	// NOTE: even after the first success, the other goroutines will still run until they finish.
	// ctx, cancel := context.WithCancel(ctx)
	// defer cancel()
	var wg errgroup.Group
	if concurrency > 0 {
		wg.SetLimit(concurrency)
	}
	for _, fn := range fns {
		fn := fn
		wg.Go(func() error {
			if ctx.Err() != nil {
				var empty T
				results <- result{empty, ctx.Err()} // TODO: is this OK?
				return nil
			}
			val, err := fn(ctx)
			select {
			case results <- result{val, err}:
			case <-ctx.Done():
			}
			return nil
		})
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	var errs ErrorSlice
	for res := range results {
		if res.err == nil {
			// NOTE: it will count as a success even if the value is the zero value (e.g. 0, "", nil, etc.)
			return res.val, nil
		}
		errs = append(errs, res.err)
		if len(errs) == len(fns) {
			break
		}
	}
	return *new(T), errs
}

func IsErrorSlice(err error) bool {
	_, ok := err.(ErrorSlice)
	return ok
}

type ErrorSlice []error

func (e ErrorSlice) Error() string {
	// format like; ErrorSlice{"error1", "error2", "error3"}
	if len(e) == 0 {
		return "ErrorSlice{}"
	}
	builder := strings.Builder{}
	builder.WriteString("ErrorSlice{")
	for i, err := range e {
		if i > 0 {
			builder.WriteString(", ")
		}
		if err == nil {
			builder.WriteString("nil")
			continue
		}
		// write quoted string
		builder.WriteString(strconv.Quote(err.Error()))
	}
	builder.WriteString("}")
	return builder.String()
}

// Filter returns a new slice of errors that satisfy the predicate.
func (e ErrorSlice) Filter(predicate func(error) bool) ErrorSlice {
	var errs ErrorSlice
	for _, err := range e {
		if predicate(err) {
			errs = append(errs, err)
		}
	}
	return errs
}

func (e ErrorSlice) All(predicate func(error) bool) bool {
	for _, err := range e {
		if !predicate(err) {
			return false
		}
	}
	return true
}

type JobFunc[T comparable] func(context.Context) (T, error)

func NewJobGroup[T comparable]() *JobGroup[T] {
	return &JobGroup[T]{}
}

type JobGroup[T comparable] []JobFunc[T]

func (r *JobGroup[T]) Add(fn JobFunc[T]) {
	*r = append(*r, fn)
}

func (r *JobGroup[T]) Run(ctx context.Context) (T, error) {
	return FirstSuccess(ctx, -1, *r...)
}

func (r *JobGroup[T]) RunWithConcurrency(ctx context.Context, concurrency int) (T, error) {
	return FirstSuccess(ctx, concurrency, *r...)
}
