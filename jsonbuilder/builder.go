package jsonbuilder

import (
	"encoding/json"
	"fmt"
	"strconv"

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
func (o *OrderedJSONObject) MarshalJSON() ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	if o.buf == nil {
		return []byte("{}"), nil
	}
	o.buf.WriteByte('}')
	data := make([]byte, o.buf.Len())
	copy(data, o.buf.Bytes())
	o.buf.B = o.buf.B[:o.buf.Len()-1] // Remove the last byte (the closing brace)
	return data, nil
}

// MarshalJSON completes the JSON array and returns its contents.
func (a *ArrayBuilder) MarshalJSON() ([]byte, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.buf == nil {
		return []byte("[]"), nil
	}
	a.buf.WriteByte(']')
	data := make([]byte, a.buf.Len())
	copy(data, a.buf.Bytes())
	a.buf.B = a.buf.B[:a.buf.Len()-1] // Remove the last byte (the closing bracket)
	return data, nil
}

func (o *OrderedJSONObject) writeKey(key string) {
	if o.err != nil {
		return
	}
	if !o.isFirst {
		o.buf.WriteByte(',')
	} else {
		o.isFirst = false
	}
	// Use the optimized string writer for the key
	o.buf.B = appendString(o.buf.B, key)
	o.buf.WriteByte(':')
}

func (a *ArrayBuilder) writeValueSeparator() {
	if a.err != nil {
		return
	}
	if !a.isFirst {
		a.buf.WriteByte(',')
	} else {
		a.isFirst = false
	}
}

// --- OrderedJSONObject Optimized Methods ---

func (o *OrderedJSONObject) String(key, value string) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf.B = appendString(o.buf.B, value)
	return o
}

func (o *OrderedJSONObject) Int(key string, value int64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf.B = strconv.AppendInt(o.buf.B, value, 10)
	return o
}

func (o *OrderedJSONObject) Uint(key string, value uint64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf.B = strconv.AppendUint(o.buf.B, value, 10)
	return o
}

func (o *OrderedJSONObject) Uint8(key string, value uint8) *OrderedJSONObject {
	return o.Uint(key, uint64(value))
}

func (o *OrderedJSONObject) Float(key string, value float64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf.B = strconv.AppendFloat(o.buf.B, value, 'f', -1, 64)
	return o
}

func (o *OrderedJSONObject) Bool(key string, value bool) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	if value {
		o.buf.WriteString("true")
	} else {
		o.buf.WriteString("false")
	}
	return o
}

func (o *OrderedJSONObject) Null(key string) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf.WriteString("null")
	return o
}

func (o *OrderedJSONObject) Raw(key string, value json.RawMessage) *OrderedJSONObject {
	return o.Value(key, value)
}

func (o *OrderedJSONObject) Object(key string, obj *OrderedJSONObject) *OrderedJSONObject {
	return o.Value(key, obj)
}

func (o *OrderedJSONObject) Array(key string, arr *ArrayBuilder) *OrderedJSONObject {
	return o.Value(key, arr)
}

// Value remains as a fallback for complex types not covered by optimized methods.
func (o *OrderedJSONObject) Value(key string, value any) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	if o.buf == nil {
		o.err = fmt.Errorf("jsonbuilder: Value called on a consumed object builder")
		return o
	}
	o.writeKey(key)
	if o.err != nil {
		return o
	}
	val, err := sonic.ConfigFastest.Marshal(value)
	if err != nil {
		o.err = fmt.Errorf("failed to marshal value for key %q: %w", key, err)
		return o
	}
	o.buf.Write(val)
	return o
}

// --- ArrayBuilder Optimized Methods ---

func (a *ArrayBuilder) AddString(value string) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf.B = appendString(a.buf.B, value)
	return a
}

func (a *ArrayBuilder) AddInt(value int64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf.B = strconv.AppendInt(a.buf.B, value, 10)
	return a
}

func (a *ArrayBuilder) AddUint(value uint64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf.B = strconv.AppendUint(a.buf.B, value, 10)
	return a
}

func (a *ArrayBuilder) AddUint8(value uint8) *ArrayBuilder {
	return a.AddUint(uint64(value))
}

func (a *ArrayBuilder) AddFloat(value float64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf.B = strconv.AppendFloat(a.buf.B, value, 'f', -1, 64)
	return a
}

func (a *ArrayBuilder) AddBool(value bool) *ArrayBuilder {
	a.writeValueSeparator()
	if value {
		a.buf.WriteString("true")
	} else {
		a.buf.WriteString("false")
	}
	return a
}

func (a *ArrayBuilder) AddNull() *ArrayBuilder {
	a.writeValueSeparator()
	a.buf.WriteString("null")
	return a
}

func (a *ArrayBuilder) AddObject(obj *OrderedJSONObject) *ArrayBuilder { return a.AddValue(obj) }
func (a *ArrayBuilder) AddArray(arr *ArrayBuilder) *ArrayBuilder       { return a.AddValue(arr) }

// AddValue remains as a fallback for complex types.
func (a *ArrayBuilder) AddValue(value any) *ArrayBuilder {
	if a.err != nil {
		return a
	}
	if a.buf == nil {
		a.err = fmt.Errorf("jsonbuilder: AddValue called on a consumed array builder")
		return a
	}
	a.writeValueSeparator()
	val, err := sonic.ConfigFastest.Marshal(value)
	if err != nil {
		a.err = fmt.Errorf("failed to marshal array value: %w", err)
		return a
	}
	a.buf.Write(val)
	return a
}

// --- Functional Methods ---

func (o *OrderedJSONObject) ObjectFunc(key string, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	obj := NewObject()
	fn(obj)
	o.Value(key, obj)
	obj.Put()
	return o
}

func (o *OrderedJSONObject) ArrayFunc(key string, fn func(*ArrayBuilder)) *OrderedJSONObject {
	arr := NewArray()
	fn(arr)
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

// appendString is a high-performance JSON string escaper.
func appendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\':
			dst = append(dst, '\\', c)
		case c >= 0x20: // Standard ASCII characters
			dst = append(dst, c)
		case c == '\n':
			dst = append(dst, '\\', 'n')
		case c == '\r':
			dst = append(dst, '\\', 'r')
		case c == '\t':
			dst = append(dst, '\\', 't')
		default: // Control characters
			dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xF])
		}
	}
	dst = append(dst, '"')
	return dst
}

var hex = "0123456789abcdef"
