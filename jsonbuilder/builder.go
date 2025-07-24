package jsonbuilder

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	"github.com/bytedance/sonic"
	"github.com/mr-tron/base58"
	"github.com/valyala/bytebufferpool"
)

// OrderedJSONObject represents a JSON object that is marshaled progressively.
type OrderedJSONObject struct {
	isFirst bool
	err     error // Holds the first error encountered.
	buf     []byte
}

// ArrayBuilder represents a JSON array that is marshaled progressively.
type ArrayBuilder struct {
	isFirst bool
	err     error // Holds the first error encountered.
	buf     []byte
}

type Recyclable interface {
	Reset() // Resets the builder's buffer, allowing it to be reused.
	Put()   // Puts the builder back into the pool.
}

// NewObject creates a new empty OrderedJSONObject, initializing its buffer.
func NewObject() *OrderedJSONObject {
	// Start with a reasonable initial capacity to reduce reallocations.
	o := getOrderedJSONObject()
	o.buf = append(o.buf, '{')
	o.isFirst = true
	return o
}

// Put resets the builder's buffer, allowing it to be reused.
// Note: This no longer uses an external pool.
func (o *OrderedJSONObject) Reset() {
	o.buf = o.buf[:0]
	o.isFirst = true
	o.err = nil
}

func (o *OrderedJSONObject) Put() {
	if o == nil {
		return
	}
	putOrderedJSONObject(o)
}

var _orderefJSONObjectPool = &sync.Pool{
	New: func() interface{} {
		return &OrderedJSONObject{
			buf:     make([]byte, 0, 512), // Initialize with a reasonable capacity.
			isFirst: true,
		}
	},
}

func putOrderedJSONObject(o *OrderedJSONObject) {
	if o == nil {
		return
	}
	o.Reset() // Reset the buffer and state.
	_orderefJSONObjectPool.Put(o)
}

func getOrderedJSONObject() *OrderedJSONObject {
	got := _orderefJSONObjectPool.Get().(*OrderedJSONObject)
	// ASSUMES that it was reset before being put into the pool.
	return got
}

var _arrayBuilderPool = &sync.Pool{
	New: func() interface{} {
		return &ArrayBuilder{
			buf:     make([]byte, 0, 256), // Initialize with a reasonable capacity.
			isFirst: true,
		}
	},
}

func getArrayBuilder() *ArrayBuilder {
	got := _arrayBuilderPool.Get().(*ArrayBuilder)
	// ASSUMES that it was reset before being put into the pool.
	return got
}

func putArrayBuilder(a *ArrayBuilder) {
	if a == nil {
		return
	}
	a.Reset() // Reset the buffer and state.
	_arrayBuilderPool.Put(a)
}

// NewArray creates a new ArrayBuilder, initializing its buffer.
func NewArray() *ArrayBuilder {
	ab := getArrayBuilder()
	ab.buf = append(ab.buf, '[')
	ab.isFirst = true
	// override the runtime finalizer to put the ArrayBuilder back to the pool
	// runtime.SetFinalizer(ab, func(a *ArrayBuilder) {
	// 	putArrayBuilder(a) // Reset the ArrayBuilder and return it to the pool.
	// })
	return ab
}

// Reset resets the builder's buffer.
func (a *ArrayBuilder) Reset() {
	a.buf = a.buf[:0]
	a.isFirst = true
	a.err = nil
}

func (a *ArrayBuilder) Put() {
	if a == nil {
		return
	}
	putArrayBuilder(a) // Reset the ArrayBuilder and return it to the pool.
}

// MarshalJSON completes the JSON object and returns its contents.
func (o *OrderedJSONObject) MarshalJSON() ([]byte, error) {
	if o.err != nil {
		return nil, o.err
	}
	if o.buf == nil {
		return []byte("{}"), nil
	}
	o.buf = append(o.buf, '}')
	data := make([]byte, len(o.buf))
	copy(data, o.buf)
	o.buf = o.buf[:len(o.buf)-1] // Truncate the last byte (the closing brace)
	return data, nil
}

func (o *OrderedJSONObject) MarshalJSONToByteBuffer() (*bytebufferpool.ByteBuffer, error) {
	if o.err != nil {
		return nil, o.err
	}
	outBuf := bytebufferpool.Get()
	if o.buf == nil {
		outBuf.B = append(outBuf.B, '{', '}')
		return outBuf, nil
	}
	o.buf = append(o.buf, '}')
	outBuf.B = append(outBuf.B, o.buf...)
	o.buf = o.buf[:len(o.buf)-1] // Truncate the last byte (the closing brace)
	return outBuf, nil
}

// MarshalJSON completes the JSON array and returns its contents.
func (a *ArrayBuilder) MarshalJSON() ([]byte, error) {
	if a.err != nil {
		return nil, a.err
	}
	if a.buf == nil {
		return []byte("[]"), nil
	}
	a.buf = append(a.buf, ']')
	data := make([]byte, len(a.buf))
	copy(data, a.buf)
	a.buf = a.buf[:len(a.buf)-1] // Truncate the last byte (the closing bracket)
	return data, nil
}

func (o *OrderedJSONObject) writeKey(key string) {
	if o.err != nil {
		return
	}
	if !o.isFirst {
		o.buf = append(o.buf, ',')
	} else {
		o.isFirst = false
	}
	// Use the optimized string writer for the key
	o.buf = appendString(o.buf, key)
	o.buf = append(o.buf, ':')
}

func (a *ArrayBuilder) writeValueSeparator() {
	if a.err != nil {
		return
	}
	if !a.isFirst {
		a.buf = append(a.buf, ',')
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
	o.buf = appendString(o.buf, value)
	return o
}

func (o *OrderedJSONObject) Base58(key string, value []byte) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = append(o.buf, '"')
	o.buf = base58.Append(o.buf, value)
	o.buf = append(o.buf, '"')
	return o
}

func (o *OrderedJSONObject) Base58Slice(key string, values [][]byte) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = append(o.buf, '[')
	for i, value := range values {
		if i > 0 {
			o.buf = append(o.buf, ',')
		}
		o.buf = append(o.buf, '"')
		o.buf = base58.Append(o.buf, value)
		o.buf = append(o.buf, '"')
	}
	o.buf = append(o.buf, ']')
	return o
}

func (o *OrderedJSONObject) Int(key string, value int64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = appendInt(o.buf, value)
	return o
}

func (o *OrderedJSONObject) Uint(key string, value uint64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = appendUint(o.buf, value)
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
	// strconv is still the most reliable for floats.
	o.buf = strconv.AppendFloat(o.buf, value, 'f', -1, 64)
	return o
}

func (o *OrderedJSONObject) Bool(key string, value bool) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	if value {
		o.buf = append(o.buf, "true"...)
	} else {
		o.buf = append(o.buf, "false"...)
	}
	return o
}

func (o *OrderedJSONObject) Null(key string) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = append(o.buf, "null"...)
	return o
}

func (o *OrderedJSONObject) EmptyArray(key string) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = append(o.buf, '[', ']')
	return o
}

// --- OrderedJSONObject Slice Methods ---

func (o *OrderedJSONObject) StringSlice(key string, values []string) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = appendStringSlice(o.buf, values)
	return o
}

func (o *OrderedJSONObject) IntSlice(key string, values []int64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = appendIntSlice(o.buf, values)
	return o
}

func (o *OrderedJSONObject) UintSlice(key string, values []uint64) *OrderedJSONObject {
	if o.err != nil {
		return o
	}
	o.writeKey(key)
	o.buf = appendUintSlice(o.buf, values)
	return o
}

// --- Fallback and Functional Methods ---

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
	o.buf = append(o.buf, val...)
	return o
}

// --- ArrayBuilder Optimized Methods ---

func (a *ArrayBuilder) AddString(value string) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = appendString(a.buf, value)
	return a
}

func (a *ArrayBuilder) AddInt(value int64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = appendInt(a.buf, value)
	return a
}

func (a *ArrayBuilder) AddUint(value uint64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = appendUint(a.buf, value)
	return a
}

func (a *ArrayBuilder) AddUint8(value uint8) *ArrayBuilder {
	return a.AddUint(uint64(value))
}

func (a *ArrayBuilder) AddFloat(value float64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = strconv.AppendFloat(a.buf, value, 'f', -1, 64)
	return a
}

func (a *ArrayBuilder) AddBool(value bool) *ArrayBuilder {
	a.writeValueSeparator()
	if value {
		a.buf = append(a.buf, "true"...)
	} else {
		a.buf = append(a.buf, "false"...)
	}
	return a
}

func (a *ArrayBuilder) AddNull() *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = append(a.buf, "null"...)
	return a
}

func (a *ArrayBuilder) AddEmptyArray() *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = append(a.buf, '[', ']')
	return a
}

// --- ArrayBuilder Slice Methods ---

func (a *ArrayBuilder) AddStringSlice(values []string) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = appendStringSlice(a.buf, values)
	return a
}

func (a *ArrayBuilder) AddIntSlice(values []int64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = appendIntSlice(a.buf, values)
	return a
}

func (a *ArrayBuilder) AddUintSlice(values []uint64) *ArrayBuilder {
	a.writeValueSeparator()
	a.buf = appendUintSlice(a.buf, values)
	return a
}

// --- Fallback and Functional Methods ---

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
	a.buf = append(a.buf, val...)
	return a
}

func (o *OrderedJSONObject) ObjectFunc(key string, fn func(*OrderedJSONObject)) *OrderedJSONObject {
	obj := NewObject()
	fn(obj)
	o.Value(key, obj)
	// No need to Put, as it's a local builder that will be garbage collected.
	return o
}

func (o *OrderedJSONObject) ArrayFunc(key string, fn func(*ArrayBuilder)) *OrderedJSONObject {
	arr := NewArray()
	fn(arr)
	o.Value(key, arr)
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

// --- Optimized Appenders ---

var hex = "0123456789abcdef"

func appendString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	i := 0
	for i < len(s) {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' {
			break
		}
		i++
	}
	if i == len(s) {
		dst = append(dst, s...)
		dst = append(dst, '"')
		return dst
	}
	dst = append(dst, s[:i]...)
	for ; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"', '\\':
			dst = append(dst, '\\', c)
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			if c < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xF])
			} else {
				dst = append(dst, c)
			}
		}
	}
	dst = append(dst, '"')
	return dst
}

func appendStringSlice(dst []byte, values []string) []byte {
	dst = append(dst, '[')
	for i, v := range values {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = appendString(dst, v)
	}
	dst = append(dst, ']')
	return dst
}

func appendIntSlice(dst []byte, values []int64) []byte {
	dst = append(dst, '[')
	for i, v := range values {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = appendInt(dst, v)
	}
	dst = append(dst, ']')
	return dst
}

func appendUintSlice(dst []byte, values []uint64) []byte {
	dst = append(dst, '[')
	for i, v := range values {
		if i > 0 {
			dst = append(dst, ',')
		}
		dst = appendUint(dst, v)
	}
	dst = append(dst, ']')
	return dst
}

// --- Integer to String Conversion ---
// Using the standard library's battle-tested strconv for correctness and reliability.

func appendUint(dst []byte, n uint64) []byte {
	return strconv.AppendUint(dst, n, 10)
}

func appendInt(dst []byte, n int64) []byte {
	return strconv.AppendInt(dst, n, 10)
}
