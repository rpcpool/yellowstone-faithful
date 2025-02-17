package jsonbuilder

import (
	"encoding/json"
	"testing"

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
