package jsonbuilder

import (
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/valyala/bytebufferpool"
)

// OrderedJSONObject represents a JSON object that is marshaled progressively.
type OrderedJSONObject struct {
	buf     *bytebufferpool.ByteBuffer
	isFirst bool
	err     error // Holds the first error encountered.
}

// ArrayBuilder represents a JSON array that is marshaled progressively.
type ArrayBuilder struct {
	buf     *bytebufferpool.ByteBuffer
	isFirst bool
	err     error // Holds the first error encountered.
}

// NewObject creates a new empty OrderedJSONObject, initializing its buffer.
func NewObject() *OrderedJSONObject {
	buf := bytebufferpool.Get()
	buf.WriteByte('{')
	return &OrderedJSONObject{
		buf:     buf,
		isFirst: true,
	}
}

// Put releases the object's buffer back to the pool. This is crucial for memory management.
func (o *OrderedJSONObject) Put() {
	if o.buf != nil {
		bytebufferpool.Put(o.buf)
		o.buf = nil // Prevent double-put
	}
}

// NewArray creates a new ArrayBuilder, initializing its buffer.
func NewArray() *ArrayBuilder {
	buf := bytebufferpool.Get()
	buf.WriteByte('[')
	return &ArrayBuilder{
		buf:     buf,
		isFirst: true,
	}
}

// Put releases the array's buffer back to the pool.
func (a *ArrayBuilder) Put() {
	if a.buf != nil {
		bytebufferpool.Put(a.buf)
		a.buf = nil // Prevent double-put
	}
}

// MarshalJSON completes the JSON object and returns its contents.
// This operation is non-destructive; the builder can be reused after this call.
// If an error occurred during building, it will be returned here.
func (o *OrderedJSONObject) MarshalJSON() ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	if o.buf == nil {
		// Builder was consumed or Put() was called.
		return []byte("{}"), nil
	}
	o.buf.WriteByte('}')
	// Create a copy to return, so the original buffer can be safely reused.
	data := make([]byte, o.buf.Len())
	copy(data, o.buf.Bytes())
	// Truncate the buffer to remove the closing brace, making it reusable.
	o.buf.B = o.buf.B[:o.buf.Len()-1]
	return data, nil
}

// MarshalJSON completes the JSON array and returns its contents.
// This operation is non-destructive; the builder can be reused after this call.
// If an error occurred during building, it will be returned here.
func (a *ArrayBuilder) MarshalJSON() ([]byte, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.buf == nil {
		// Builder was consumed or Put() was called.
		return []byte("[]"), nil
	}
	a.buf.WriteByte(']')
	// Create a copy to return, so the original buffer can be safely reused.
	data := make([]byte, a.buf.Len())
	copy(data, a.buf.Bytes())
	// Truncate the buffer to remove the closing brace, making it reusable.
	a.buf.B = a.buf.B[:a.buf.Len()-1]
	return data, nil
}

// writeKey handles writing the comma and the marshaled key to the buffer.
func (o *OrderedJSONObject) writeKey(key string) {
	// No-op if an error has already occurred.
	if o.err != nil {
		return
	}

	if !o.isFirst {
		o.buf.WriteByte(',')
	} else {
		o.isFirst = false
	}

	keyBytes, err := sonic.ConfigFastest.Marshal(key)
	if err != nil {
		o.err = fmt.Errorf("failed to marshal key %q: %w", key, err)
		return
	}
	o.buf.Write(keyBytes)
	o.buf.WriteByte(':')
}

// writeValue handles writing the comma for an array element.
func (a *ArrayBuilder) writeValue() {
	// No-op if an error has already occurred.
	if a.err != nil {
		return
	}
	if !a.isFirst {
		a.buf.WriteByte(',')
	} else {
		a.isFirst = false
	}
}

// Value adds a generic JSON value to the object, marshaling it immediately.
func (o *OrderedJSONObject) Value(key string, value any) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	// Prevent panic on already-consumed builders.
	if o.buf == nil {
		o.err = fmt.Errorf("jsonbuilder: Value called on a consumed object builder")
		return o
	}

	o.writeKey(key)
	// writeKey might have set an error.
	if o.err != nil {
		return o
	}

	switch v := value.(type) {
	case *OrderedJSONObject:
		if v.err != nil {
			o.err = v.err
			return o
		}
		// Marshal the nested object. MarshalJSON is non-destructive.
		// The caller who created 'v' is responsible for calling Put() on it.
		val, err := v.MarshalJSON()
		if err != nil {
			o.err = err
			return o
		}
		o.buf.Write(val)
	case *ArrayBuilder:
		if v.err != nil {
			o.err = v.err
			return o
		}
		// Marshal the nested array. MarshalJSON is non-destructive.
		val, err := v.MarshalJSON()
		if err != nil {
			o.err = err
			return o
		}
		o.buf.Write(val)
	default:
		val, err := sonic.ConfigFastest.Marshal(v)
		if err != nil {
			o.err = fmt.Errorf("failed to marshal value for key %q: %w", key, err)
			return o
		}
		o.buf.Write(val)
	}
	return o
}

// AddValue appends a generic value to the array, marshaling it immediately.
func (a *ArrayBuilder) AddValue(value any) *ArrayBuilder {
	if a.err != nil {
		return a
	}
	// Prevent panic on already-consumed builders.
	if a.buf == nil {
		a.err = fmt.Errorf("jsonbuilder: AddValue called on a consumed array builder")
		return a
	}

	a.writeValue()

	switch v := value.(type) {
	case *OrderedJSONObject:
		if v.err != nil {
			a.err = v.err
			return a
		}
		val, err := v.MarshalJSON()
		if err != nil {
			a.err = err
			return a
		}
		a.buf.Write(val)
	case *ArrayBuilder:
		if v.err != nil {
			a.err = v.err
			return a
		}
		val, err := v.MarshalJSON()
		if err != nil {
			a.err = err
			return a
		}
		a.buf.Write(val)
	default:
		val, err := sonic.ConfigFastest.Marshal(v)
		if err != nil {
			a.err = fmt.Errorf("failed to marshal array value: %w", err)
			return a
		}
		a.buf.Write(val)
	}
	return a
}

// --- OrderedJSONObject Methods ---

func (o *OrderedJSONObject) String(key, value string) *OrderedJSONObject { return o.Value(key, value) }

func (o *OrderedJSONObject) Int(key string, value int64) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Uint(key string, value uint64) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Uint8(key string, value uint8) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Float(key string, value float64) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Bool(key string, value bool) *OrderedJSONObject {
	return o.Value(key, value)
}
func (o *OrderedJSONObject) Null(key string) *OrderedJSONObject { return o.Value(key, nil) }
func (o *OrderedJSONObject) Raw(key string, value json.RawMessage) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Object(key string, obj *OrderedJSONObject) *OrderedJSONObject {
	return o.Value(key, obj)
}

func (o *OrderedJSONObject) Array(key string, arr *ArrayBuilder) *OrderedJSONObject {
	return o.Value(key, arr)
}

func (o *OrderedJSONObject) ObjectFunc(key string, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	obj := NewObject()
	fn(obj)
	// Value is now non-consuming, so we must explicitly Put the temporary object.
	o.Value(key, obj)
	obj.Put()
	return o
}

func (o *OrderedJSONObject) ArrayFunc(key string, fn func(*ArrayBuilder)) *OrderedJSONObject {
	arr := NewArray()
	fn(arr)
	// Value is now non-consuming, so we must explicitly Put the temporary array.
	o.Value(key, arr)
	arr.Put()
	return o
}

func (o *OrderedJSONObject) Func(fn func(*OrderedJSONObject)) *OrderedJSONObject {
	if o.err == nil && fn != nil {
		fn(o)
	}
	return o
}

func (o *OrderedJSONObject) Apply(key string, fn func() any) *OrderedJSONObject {
	if o.err == nil && fn != nil {
		return o.Value(key, fn())
	}
	return o
}

func (o *OrderedJSONObject) ApplyIf(condition bool, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	if o.err == nil && condition && fn != nil {
		fn(o)
	}
	return o
}

// --- ArrayBuilder Methods ---

func (a *ArrayBuilder) AddString(value string) *ArrayBuilder           { return a.AddValue(value) }
func (a *ArrayBuilder) AddUint8(value uint8) *ArrayBuilder             { return a.AddValue(value) }
func (a *ArrayBuilder) AddUint(value uint64) *ArrayBuilder             { return a.AddValue(value) }
func (a *ArrayBuilder) AddInt(value int64) *ArrayBuilder               { return a.AddValue(value) }
func (a *ArrayBuilder) AddFloat(value float64) *ArrayBuilder           { return a.AddValue(value) }
func (a *ArrayBuilder) AddBool(value bool) *ArrayBuilder               { return a.AddValue(value) }
func (a *ArrayBuilder) AddObject(obj *OrderedJSONObject) *ArrayBuilder { return a.AddValue(obj) }
func (a *ArrayBuilder) AddArray(arr *ArrayBuilder) *ArrayBuilder       { return a.AddValue(arr) }
func (a *ArrayBuilder) AddNull() *ArrayBuilder                         { return a.AddValue(nil) }

func (a *ArrayBuilder) Func(fn func(*ArrayBuilder)) *ArrayBuilder {
	if a.err == nil && fn != nil {
		fn(a)
	}
	return a
}

func (a *ArrayBuilder) Apply(fn func() any) *ArrayBuilder {
	if a.err == nil && fn != nil {
		a.AddValue(fn())
	}
	return a
}

func (a *ArrayBuilder) ApplyIf(condition bool, fn func(*ArrayBuilder)) *ArrayBuilder {
	if a.err == nil && condition && fn != nil {
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
		ObjectFunc("address", func(o *OrderedJSONObject) {
			o.String("street", "123 Main St")
			o.Apply("city", func() any {
				return "Springfield"
			})
		}).
		ArrayFunc("hobbies", func(a *ArrayBuilder) {
			a.AddString("reading")
			a.AddString("hiking")
			a.Apply(func() any {
				return NewObject().
					String("name", "cooking").
					Int("years", 5)
			})
		})

	// Use json.Marshal, which will call our custom MarshalJSON method.
	jsonData, err := json.Marshal(person)
	if err != nil {
		fmt.Printf("Error during build: %v\n", err)
	} else {
		fmt.Println(string(jsonData))
	}

	// It's critical to Put the top-level object back to the pool when done.
	person.Put()
}

func Example_build_with_error() {
	// Create an invalid object to trigger an error.
	// A map[string]interface{} with a non-string key is not valid JSON.
	invalidValue := map[interface{}]string{
		123: "value",
	}

	obj := NewObject().
		String("field1", "ok").
		Value("invalid_field", invalidValue). // This will cause a marshaling error.
		String("field2", "this will not be added")

	_, err := json.Marshal(obj)
	if err != nil {
		fmt.Printf("Successfully caught error: %v\n", err)
	} else {
		fmt.Println("Failed to catch error.")
	}

	obj.Put()
}
