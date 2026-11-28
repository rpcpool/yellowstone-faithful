package jsonbuilder

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestObjectResetReturnsEmptyJSON(t *testing.T) {
	obj := NewObject().String("old", "value")
	defer obj.Put()

	obj.Reset()
	data, err := obj.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(data))
}

func TestObjectResetCanBeReused(t *testing.T) {
	obj := NewObject().String("old", "value")
	defer obj.Put()

	obj.Reset()
	obj.String("new", "value").Int("count", 2)
	data, err := obj.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `{"new":"value","count":2}`, string(data))
}

func TestArrayResetReturnsEmptyJSON(t *testing.T) {
	arr := NewArray().AddString("old")
	defer arr.Put()

	arr.Reset()
	data, err := arr.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(data))
}

func TestArrayResetCanBeReused(t *testing.T) {
	arr := NewArray().AddString("old")
	defer arr.Put()

	arr.Reset()
	arr.AddString("new").AddInt(2)
	data, err := arr.MarshalJSON()
	require.NoError(t, err)
	require.JSONEq(t, `["new",2]`, string(data))
}
func TestResetClearsMarshalError(t *testing.T) {
	t.Run("object", func(t *testing.T) {
		obj := NewObject().Value("unsupported", func() {})
		defer obj.Put()
		_, err := obj.MarshalJSON()
		require.Error(t, err)

		obj.Reset()
		obj.Reset()
		obj.String("new", "value")
		data, err := obj.MarshalJSON()
		require.NoError(t, err)
		require.JSONEq(t, `{"new":"value"}`, string(data))
	})

	t.Run("array", func(t *testing.T) {
		arr := NewArray().AddValue(func() {})
		defer arr.Put()
		_, err := arr.MarshalJSON()
		require.Error(t, err)

		arr.Reset()
		arr.Reset()
		arr.AddString("new")
		data, err := arr.MarshalJSON()
		require.NoError(t, err)
		require.JSONEq(t, `["new"]`, string(data))
	})
}

func TestResetPoolLifecycle(t *testing.T) {
	for range 100 {
		obj := NewObject().String("old", "value")
		obj.Put()
		obj = NewObject()
		data, err := obj.MarshalJSON()
		require.NoError(t, err)
		require.JSONEq(t, `{}`, string(data))
		obj.Put()

		arr := NewArray().AddString("old")
		arr.Put()
		arr = NewArray()
		data, err = arr.MarshalJSON()
		require.NoError(t, err)
		require.JSONEq(t, `[]`, string(data))
		arr.Put()
	}
}
