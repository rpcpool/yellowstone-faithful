package jsonbuilder

import (
	"bytes"
	"encoding/json"
	"fmt"
)

type OrderedJSONObject struct {
	fields []field
}

type field struct {
	key   string
	value any
}

type ArrayBuilder struct {
	elements []any
}

// ArrayBuilder.MarshalJSON()
func (a *ArrayBuilder) MarshalJSON() ([]byte, error) {
	if len(a.elements) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal(a.elements)
}

// Object methods
func NewObject() *OrderedJSONObject {
	return &OrderedJSONObject{}
}

func (o *OrderedJSONObject) String(key, value string) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, value})
	return o
}

func (o *OrderedJSONObject) Int(key string, value int64) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, value})
	return o
}

func (o *OrderedJSONObject) Float(key string, value float64) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, value})
	return o
}

func (o *OrderedJSONObject) Bool(key string, value bool) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, value})
	return o
}

func (o *OrderedJSONObject) Object(key string, obj *OrderedJSONObject) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, obj})
	return o
}

func (o *OrderedJSONObject) Array(key string, arr *ArrayBuilder) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, arr.elements})
	return o
}

func (o *OrderedJSONObject) Null(key string) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, nil})
	return o
}

// Functional composition methods for objects
func (o *OrderedJSONObject) Func(fn func(*OrderedJSONObject)) *OrderedJSONObject {
	fn(o)
	return o
}

func (o *OrderedJSONObject) Apply(key string, fn func() any) *OrderedJSONObject {
	o.fields = append(o.fields, field{key, fn()})
	return o
}

func (o *OrderedJSONObject) ApplyIf(condition bool, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	if condition {
		fn(o)
	}
	return o
}

// Array methods
func NewArray() *ArrayBuilder {
	return &ArrayBuilder{}
}

func (a *ArrayBuilder) AddString(value string) *ArrayBuilder {
	a.elements = append(a.elements, value)
	return a
}

func (a *ArrayBuilder) AddInt(value int64) *ArrayBuilder {
	a.elements = append(a.elements, value)
	return a
}

func (a *ArrayBuilder) AddFloat(value float64) *ArrayBuilder {
	a.elements = append(a.elements, value)
	return a
}

func (a *ArrayBuilder) AddBool(value bool) *ArrayBuilder {
	a.elements = append(a.elements, value)
	return a
}

func (a *ArrayBuilder) AddObject(obj *OrderedJSONObject) *ArrayBuilder {
	a.elements = append(a.elements, obj)
	return a
}

func (a *ArrayBuilder) AddArray(arr *ArrayBuilder) *ArrayBuilder {
	a.elements = append(a.elements, arr.elements)
	return a
}

func (a *ArrayBuilder) AddNull() *ArrayBuilder {
	a.elements = append(a.elements, nil)
	return a
}

// Functional composition methods for arrays
func (a *ArrayBuilder) Func(fn func(*ArrayBuilder)) *ArrayBuilder {
	fn(a)
	return a
}

func (a *ArrayBuilder) Apply(fn func() any) *ArrayBuilder {
	a.elements = append(a.elements, fn())
	return a
}

func (a *ArrayBuilder) ApplyIf(condition bool, fn func(*ArrayBuilder)) *ArrayBuilder {
	if condition {
		fn(a)
	}
	return a
}

// Marshaling implementation
func (o *OrderedJSONObject) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')

	for i, f := range o.fields {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(f.key)
		buf.Write(key)
		buf.WriteByte(':')
		val, _ := json.Marshal(f.value)
		buf.Write(val)
	}

	buf.WriteByte('}')
	return buf.Bytes(), nil
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
