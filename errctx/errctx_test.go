package errctx

import (
	"errors"
	"fmt"
	"testing"
)

func TestWrap(t *testing.T) {
	t.Run("returns nil on nil input", func(t *testing.T) {
		err := Wrap(nil, "somewhere")
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("correctly formats error string", func(t *testing.T) {
		inner := errors.New("database connection failed")
		err := Wrap(inner, "persistence_layer")
		expected := "error at persistence_layer: database connection failed"
		if err.Error() != expected {
			t.Errorf("expected %q, got %q", expected, err.Error())
		}
	})
}

func TestUnwrapAndIs(t *testing.T) {
	sentinel := errors.New("unauthorized")
	err := Wrap(sentinel, "auth_middleware")

	if !errors.Is(err, sentinel) {
		t.Error("errors.Is failed to find sentinel error through Wrap")
	}

	unwrapped := errors.Unwrap(err)
	if unwrapped != sentinel {
		t.Errorf("expected %v, got %v", sentinel, unwrapped)
	}
}

func TestGetLocation(t *testing.T) {
	t.Run("retrieves location from flat wrap", func(t *testing.T) {
		err := Wrap(errors.New("fail"), "service_x")
		loc := GetLocation(err)
		if loc != "service_x" {
			t.Errorf("expected service_x, got %q", loc)
		}
	})

	t.Run("retrieves location from nested chain", func(t *testing.T) {
		inner := Wrap(errors.New("fail"), "deep_logic")
		outer := fmt.Errorf("context: %w", inner)

		loc := GetLocation(outer)
		if loc != "deep_logic" {
			t.Errorf("expected deep_logic, got %q", loc)
		}
	})

	t.Run("returns empty string when missing", func(t *testing.T) {
		err := errors.New("standard error")
		loc := GetLocation(err)
		if loc != "" {
			t.Errorf("expected empty string, got %q", loc)
		}
	})
}
