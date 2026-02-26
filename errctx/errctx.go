// Package errctx provides utilities for adding location context to Go errors.
package errctx

import (
	"errors"
	"fmt"
)

// ErrorWithLocation attaches a specific string location or context tag to an error.
type ErrorWithLocation struct {
	Err      error
	Location string
}

func (e *ErrorWithLocation) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("error at %s: %v", e.Location, e.Err)
}

func (e *ErrorWithLocation) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// Wrap adds location context to an error. Returns nil if err is nil.
func Wrap(err error, location string) error {
	if err == nil {
		return nil
	}
	return &ErrorWithLocation{
		Err:      err,
		Location: location,
	}
}

// GetLocation retrieves the location string from an error chain if it exists.
// Returns an empty string if no ErrorWithLocation is found.
func GetLocation(err error) string {
	var target *ErrorWithLocation
	if errors.As(err, &target) {
		return target.Location
	}
	return ""
}
