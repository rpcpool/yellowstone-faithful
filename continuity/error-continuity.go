// Package continuity allows to chain calls that continue if there's no error, or
// stop if there's an error. Each call returns a Continuation object that can be used to
// chain the next call.
package continuity

import "strings"

type IfThen struct {
	failedAt ErrArray
}

type ErrArray []error

func (e ErrArray) Error() string {
	if len(e) == 0 {
		return ""
	}
	if len(e) == 1 {
		return e[0].Error()
	}
	errs := make([]string, len(e))
	for i, err := range e {
		errs[i] = err.Error()
	}
	return "multiple errors: " + strings.Join(errs, ", ")
}

func New() *IfThen {
	return new(IfThen)
}

func (it *IfThen) Thenf(name string, f func() error) *IfThen {
	if len(it.failedAt) > 0 {
		return it
	}
	err := f()
	if err != nil {
		it.failedAt = append(it.failedAt, err)
	}
	return it
}

func (it *IfThen) Then(name string, errs ...error) *IfThen {
	if len(it.failedAt) > 0 {
		return it
	}
	nonNil := getNonNil(errs...)
	if len(nonNil) > 0 {
		it.failedAt = append(it.failedAt, nonNil...)
	}
	return it
}

func getNonNil(errs ...error) []error {
	var nonNil []error
	for _, err := range errs {
		if err != nil {
			nonNil = append(nonNil, err)
		}
	}
	return nonNil
}

func (it *IfThen) Err() error {
	if len(it.failedAt) == 0 {
		return nil
	}
	return it.failedAt
}

func example_if_then() error {
	chain := New()
	return chain.
		Thenf("Step 1", func() error {
			// Do something
			return nil
		}).
		Thenf("Step 2", func() error {
			// Do something else
			return nil
		}).Err()
}
