package jsonbuilder

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type JSONBuilderTestSuite struct {
	suite.Suite
}

func TestJSONBuilderSuite(t *testing.T) {
	suite.Run(t, new(JSONBuilderTestSuite))
}

func (s *JSONBuilderTestSuite) TestBasicObject() {
	obj := NewObject().
		String("name", "Alice").
		Int("age", 30).
		Bool("active", true)

	expected := `{"name":"Alice","age":30,"active":true}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestNestedObjects() {
	address := NewObject().
		String("street", "123 Main St").
		String("city", "Springfield")

	person := NewObject().
		String("name", "Bob").
		Object("address", address)

	expected := `{"name":"Bob","address":{"street":"123 Main St","city":"Springfield"}}`
	result, _ := json.Marshal(person)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestArrays() {
	hobbies := NewArray().
		AddString("reading").
		AddString("hiking")

	person := NewObject().
		String("name", "Charlie").
		Array("hobbies", hobbies)

	expected := `{"name":"Charlie","hobbies":["reading","hiking"]}`
	result, _ := json.Marshal(person)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestFuncMethod() {
	obj := NewObject().
		Func(func(o *OrderedJSONObject) {
			o.String("name", "Dave")
			o.Int("age", 40)
		})

	expected := `{"name":"Dave","age":40}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestApplyMethod() {
	obj := NewObject().
		Apply("timestamp", func() interface{} {
			return "2023-07-20T12:34:56Z"
		}).
		Apply("dynamic", func() interface{} {
			return NewObject().String("key", "value")
		})

	expected := `{"timestamp":"2023-07-20T12:34:56Z","dynamic":{"key":"value"}}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestApplyIf() {
	obj := NewObject().
		String("name", "Eve").
		ApplyIf(true, func(o *OrderedJSONObject) {
			o.Bool("verified", true)
		}).
		ApplyIf(false, func(o *OrderedJSONObject) {
			o.String("shouldNotExist", "value")
		})

	expected := `{"name":"Eve","verified":true}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestArrayFunc() {
	arr := NewArray().
		Func(func(a *ArrayBuilder) {
			a.AddString("first")
			a.AddInt(42)
		}).
		AddObject(NewObject().String("key", "value"))

	expected := `["first",42,{"key":"value"}]`
	result, _ := json.Marshal(arr.elements)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestComplexStructure() {
	obj := NewObject().
		Func(func(o *OrderedJSONObject) {
			o.String("name", "Frank")
			o.Array("scores", NewArray().
				Func(func(a *ArrayBuilder) {
					a.AddInt(95)
					a.AddInt(88)
				}))
		}).
		Apply("metadata", func() interface{} {
			return NewObject().
				String("created", "2023-01-01").
				Bool("public", true)
		})

	expected := `{
		"name": "Frank",
		"scores": [95, 88],
		"metadata": {
			"created": "2023-01-01",
			"public": true
		}
	}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestEmptyObject() {
	obj := NewObject()
	expected := `{}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestEmptyArray() {
	arr := NewArray()
	expected := `[]`
	result, _ := arr.MarshalJSON()
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestOrderPreservation() {
	obj := NewObject().
		String("a", "1").
		String("c", "3").
		String("b", "2")

	expected := `{"a":"1","c":"3","b":"2"}`
	result, _ := json.Marshal(obj)
	s.JSONEq(expected, string(result))
}

func (s *JSONBuilderTestSuite) TestNullHandling() {
	obj := NewObject().
		Null("nullField").
		Array("nullArray", NewArray().AddNull())

	expected := `{
		"nullField": null,
		"nullArray": [null]
	}`
	result, err := json.Marshal(obj)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestInvalidJSONValues() {
	// Channel cannot be marshaled to JSON
	invalidObj := NewObject().Value("invalid", make(chan int))
	_, err := json.Marshal(invalidObj)
	assert.Error(s.T(), err)
	assert.Contains(s.T(), err.Error(), "failed to marshal value for key \"invalid\"")
}

func (s *JSONBuilderTestSuite) TestDeepNesting() {
	deepObj := NewObject()
	current := deepObj

	// Create 10-level nested object
	for i := 0; i < 10; i++ {
		nested := NewObject().Int("level", int64(i))
		current.Object("nested", nested)
		current = nested
	}

	result, err := json.Marshal(deepObj)
	require.NoError(s.T(), err)

	expected := `{"nested":{"level":0,"nested":{"level":1,"nested":{"level":2,"nested":{"level":3,"nested":{"level":4,"nested":{"level":5,"nested":{"level":6,"nested":{"level":7,"nested":{"level":8,"nested":{"level":9}}}}}}}}}}}`
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestLargeStructure() {
	arr := NewArray()

	// Build array with 1000 elements
	for i := 0; i < 1000; i++ {
		arr.AddInt(int64(i))
	}

	result, err := json.Marshal(arr)
	require.NoError(s.T(), err)

	var parsed []int
	require.NoError(s.T(), json.Unmarshal(result, &parsed))
	assert.Len(s.T(), parsed, 1000)
	assert.Equal(s.T(), 999, parsed[999])
}

func (s *JSONBuilderTestSuite) TestReusedBuilder() {
	builder := NewObject().String("base", "value")

	// First use
	result1, err := json.Marshal(builder)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), `{"base":"value"}`, string(result1))

	// Reuse with new fields
	builder.Int("count", 42)
	result2, err := json.Marshal(builder)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), `{"base":"value","count":42}`, string(result2))
}

func (s *JSONBuilderTestSuite) TestSpecialCharacters() {
	obj := NewObject().
		String("key\nwith\tchars", "value\"with\\chars").
		Array("emoji", NewArray().AddString("😀🎉"))

	expected := `{
		"key\nwith\tchars": "value\"with\\chars",
		"emoji": ["😀🎉"]
	}`
	result, err := json.Marshal(obj)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestMixedArrayTypes() {
	arr := NewArray().
		AddString("text").
		AddInt(42).
		AddFloat(3.14).
		AddBool(true).
		AddNull().
		AddObject(NewObject().String("key", "value")).
		AddArray(NewArray().AddInt(1))

	expected := `[
		"text",
		42,
		3.14,
		true,
		null,
		{"key":"value"},
		[1]
	]`
	result, err := json.Marshal(arr)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestChainedConditionals() {
	obj := NewObject().
		ApplyIf(false, func(o *OrderedJSONObject) {
			o.String("never", "added")
		}).
		ApplyIf(true, func(o *OrderedJSONObject) {
			o.ApplyIf(1 == 2, func(o *OrderedJSONObject) {
				o.String("nope", "no")
			}).
				ApplyIf(2 == 2, func(o *OrderedJSONObject) {
					o.String("yes", "yes")
				})
		})

	expected := `{"yes":"yes"}`
	result, err := json.Marshal(obj)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestEmptyKeyName() {
	obj := NewObject().String("", "empty key")
	expected := `{"":"empty key"}`
	result, err := json.Marshal(obj)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestRawJSONValue() {
	obj := NewObject().
		Value("raw", json.RawMessage(`{"sub":"value"}`))

	expected := `{"raw":{"sub":"value"}}`
	result, err := json.Marshal(obj)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}

func (s *JSONBuilderTestSuite) TestConcurrentUsage() {
	const numGoroutines = 100
	results := make(chan string, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			obj := NewObject().
				Int("id", int64(n)).
				String("goroutine", "yes")
			data, _ := json.Marshal(obj)
			results <- string(data)
		}(i)
	}

	unique := make(map[string]struct{})
	for i := 0; i < numGoroutines; i++ {
		data := <-results
		unique[data] = struct{}{}
	}

	assert.Equal(s.T(), numGoroutines, len(unique), "all generated JSON should be unique")
}

func (s *JSONBuilderTestSuite) TestComplexNestedConditionals() {
	obj := NewObject().
		Func(func(o *OrderedJSONObject) {
			o.Apply("dynamic", func() any {
				return NewArray().
					ApplyIf(true, func(a *ArrayBuilder) {
						a.AddObject(NewObject().
							ApplyIf(false, func(o *OrderedJSONObject) {
								o.String("hidden", "value")
							}).
							Bool("visible", true))
					})
			})
		})

	expected := `{"dynamic":[{"visible":true}]}`
	result, err := json.Marshal(obj)
	require.NoError(s.T(), err)
	assert.JSONEq(s.T(), expected, string(result))
}
