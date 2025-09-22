package jsonbuilder

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAppendString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty", "", `""`},
		{"Simple", "hello", `"hello"`},
		{"With Quotes", `hello"world`, `"hello\"world"`},
		{"With Backslash", `hello\world`, `"hello\\world"`},
		{"With Newline", "hello\nworld", `"hello\nworld"`},
		{"With Tab", "hello\tworld", `"hello\tworld"`},
		{"With Carriage Return", "hello\rworld", `"hello\rworld"`},
		{"With Mixed Special Chars", `"\n\t\r\\`, `"\"\\n\\t\\r\\\\"`},
		{"With Control Chars", "a\x00b", `"a\u0000b"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendString(nil, tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestAppendInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"Zero", 0, "0"},
		{"Positive", 12345, "12345"},
		{"Negative", -54321, "-54321"},
		{"MaxInt64", math.MaxInt64, "9223372036854775807"},
		{"MinInt64", math.MinInt64, "-9223372036854775808"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendInt(nil, tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestAppendUint(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected string
	}{
		{"Zero", 0, "0"},
		{"Positive", 12345, "12345"},
		{"MaxUint64", math.MaxUint64, "18446744073709551615"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendUint(nil, tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestAppendStringSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"Nil", nil, "[]"},
		{"Empty", []string{}, "[]"},
		{"Single", []string{"a"}, `["a"]`},
		{"Multiple", []string{"a", "b", "c"}, `["a","b","c"]`},
		{"With Special Chars", []string{"a\"", "b\\"}, `["a\"","b\\"]`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendStringSlice(nil, tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestAppendIntSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []int64
		expected string
	}{
		{"Nil", nil, "[]"},
		{"Empty", []int64{}, "[]"},
		{"Single", []int64{1}, "[1]"},
		{"Multiple", []int64{1, -2, 300}, "[1,-2,300]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendIntSlice(nil, tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}

func TestAppendUintSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []uint64
		expected string
	}{
		{"Nil", nil, "[]"},
		{"Empty", []uint64{}, "[]"},
		{"Single", []uint64{1}, "[1]"},
		{"Multiple", []uint64{1, 2, 300}, "[1,2,300]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := appendUintSlice(nil, tt.input)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
