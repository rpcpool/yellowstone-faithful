package jsonbuilder

import (
	"bytes"
	"encoding/json"
	"fmt"

	jsoniter "github.com/json-iterator/go"
)

// OrderedJSONObject represents a JSON object that maintains field insertion order
type OrderedJSONObject struct {
	fields []field
}

type field struct {
	key   string
	value any
}

// ArrayBuilder represents a JSON array builder that maintains element order
type ArrayBuilder struct {
	elements []any
}

// NewObject creates a new empty OrderedJSONObject
func NewObject() *OrderedJSONObject {
	return &OrderedJSONObject{}
}

var jsonCustom = jsoniter.ConfigCompatibleWithStandardLibrary

// MarshalJSON implements custom JSON marshaling with order preservation
func (o *OrderedJSONObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	for i, f := range o.fields {
		if i > 0 {
			buf.WriteByte(',')
		}

		key, err := jsonCustom.Marshal(f.key)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal key %q: %w", f.key, err)
		}
		buf.Write(key)
		buf.WriteByte(':')

		val, err := jsonCustom.Marshal(f.value)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal value for key %q: %w", f.key, err)
		}
		buf.Write(val)
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// MarshalJSON implements JSON marshaling for arrays
func (a *ArrayBuilder) MarshalJSON() ([]byte, error) {
	return jsonCustom.Marshal(a.elements)
}

// Value adds a generic JSON value to the object
func (o *OrderedJSONObject) Value(key string, value any) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, value})
	return o
}

// String adds a string field to the object
func (o *OrderedJSONObject) String(key, value string) *OrderedJSONObject {
	return o.Value(key, value)
}

// Int adds an integer field to the object
func (o *OrderedJSONObject) Int(key string, value int64) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Uint(key string, value uint64) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Uint8(key string, value uint8) *OrderedJSONObject {
	return o.Value(key, value)
}

// Float adds a float field to the object
func (o *OrderedJSONObject) Float(key string, value float64) *OrderedJSONObject {
	return o.Value(key, value)
}

// Bool adds a boolean field to the object
func (o *OrderedJSONObject) Bool(key string, value bool) *OrderedJSONObject {
	return o.Value(key, value)
}

// Object adds a nested JSON object
func (o *OrderedJSONObject) Object(key string, obj *OrderedJSONObject) *OrderedJSONObject {
	return o.Value(key, obj)
}

func (o *OrderedJSONObject) ObjectFunc(key string, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	if obj := NewObject(); obj != nil {
		fn(obj)
		return o.Object(key, obj)
	}
	return o
}

// Array adds a JSON array field
func (o *OrderedJSONObject) Array(key string, arr *ArrayBuilder) *OrderedJSONObject {
	return o.Value(key, arr)
}

func (o *OrderedJSONObject) ArrayFunc(key string, fn func(*ArrayBuilder)) *OrderedJSONObject {
	if arr := NewArray(); arr != nil {
		fn(arr)
		return o.Array(key, arr)
	}
	return o
}

// Null adds a null field to the object
func (o *OrderedJSONObject) Null(key string) *OrderedJSONObject {
	return o.Value(key, nil)
}

func (o *OrderedJSONObject) Raw(key string, value json.RawMessage) *OrderedJSONObject {
	return o.Value(key, value)
}

// Func applies a function to modify the object
func (o *OrderedJSONObject) Func(fn func(*OrderedJSONObject)) *OrderedJSONObject {
	if fn != nil {
		fn(o)
	}
	return o
}

// Apply conditionally adds a field using a value-generating function;
// the field is only added if the function returns a non-nil value.
func (o *OrderedJSONObject) Apply(key string, fn func() any) *OrderedJSONObject {
	if fn != nil {
		return o.Value(key, fn())
	}
	return o
}

// ApplyIf conditionally applies a builder function
func (o *OrderedJSONObject) ApplyIf(condition bool, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	if condition && fn != nil {
		fn(o)
	}
	return o
}

// NewArray creates a new ArrayBuilder
func NewArray() *ArrayBuilder {
	return &ArrayBuilder{
		elements: make([]any, 0),
	}
}

// AddValue appends a generic value to the array
func (a *ArrayBuilder) AddValue(value any) *ArrayBuilder {
	a.elements = append(a.elements, value)
	return a
}

// AddString appends a string value to the array
func (a *ArrayBuilder) AddString(value string) *ArrayBuilder {
	return a.AddValue(value)
}

func (a *ArrayBuilder) AddUint8(value uint8) *ArrayBuilder {
	return a.AddValue(value)
}

func (a *ArrayBuilder) AddUint(value uint64) *ArrayBuilder {
	return a.AddValue(value)
}

// AddInt appends an integer value to the array
func (a *ArrayBuilder) AddInt(value int64) *ArrayBuilder {
	return a.AddValue(value)
}

// AddFloat appends a float value to the array
func (a *ArrayBuilder) AddFloat(value float64) *ArrayBuilder {
	return a.AddValue(value)
}

// AddBool appends a boolean value to the array
func (a *ArrayBuilder) AddBool(value bool) *ArrayBuilder {
	return a.AddValue(value)
}

// AddObject appends a JSON object to the array
func (a *ArrayBuilder) AddObject(obj *OrderedJSONObject) *ArrayBuilder {
	return a.AddValue(obj)
}

// AddArray appends another array to the array
func (a *ArrayBuilder) AddArray(arr *ArrayBuilder) *ArrayBuilder {
	return a.AddValue(arr)
}

// AddNull appends a null value to the array
func (a *ArrayBuilder) AddNull() *ArrayBuilder {
	return a.AddValue(nil)
}

// Func applies a function to modify the array
func (a *ArrayBuilder) Func(fn func(*ArrayBuilder)) *ArrayBuilder {
	if fn != nil {
		fn(a)
	}
	return a
}

// Apply appends a value generated by a function
func (a *ArrayBuilder) Apply(fn func() any) *ArrayBuilder {
	if fn != nil {
		a.AddValue(fn())
	}
	return a
}

// ApplyIf conditionally applies a builder function
func (a *ArrayBuilder) ApplyIf(condition bool, fn func(*ArrayBuilder)) *ArrayBuilder {
	if condition && fn != nil {
		fn(a)
	}
	return a
}

func Example_build() {
	// Example usage with functional composition
	person := NewObject().
		Func(func(o *OrderedJSONObject) {
			o.String("name", "John Doe")
			o.Int("age", 30)
		}).
		Apply("timestamp", func() any {
			return "2023-07-20T12:34:56Z"
		}).
		ApplyIf(true, func(o *OrderedJSONObject) {
			o.Bool("verified", true)
		}).
		Object("address", NewObject().
			Func(func(o *OrderedJSONObject) {
				o.String("street", "123 Main St")
				o.Apply("city", func() any {
					return "Springfield"
				})
			})).
		Array("hobbies", NewArray().
			Func(func(a *ArrayBuilder) {
				a.AddString("reading")
				a.AddString("hiking")
			}).
			Apply(func() any {
				return NewObject().
					String("name", "cooking").
					Int("years", 5)
			}))

	jsonData, _ := json.MarshalIndent(person, "", "  ")
	fmt.Println(string(jsonData))
}
