package validator

import (
	"encoding/json"
	"math"
	"math/big"
	"strings"
	"testing"
)

// Hostile, degenerate and boundary inputs. Nothing here may panic, and every
// case must produce a definite answer.
//
// A few of these encode deliberate choices rather than obvious answers —
// rune-based lengths, NaN and Infinity failing the integer rules, and a dotted
// key asserting its intermediates are objects. Each is noted at the case.

// --- degenerate rules -----------------------------------------------------

func TestDegenerateRules(t *testing.T) {
	t.Run("an empty rule string checks nothing", func(t *testing.T) {
		ok(t, Rules{"f": ""}, Data{"f": 1})
	})
	t.Run("an empty token between separators is skipped", func(t *testing.T) {
		ok(t, Rules{"f": "string||min:2"}, Data{"f": "abc"})
	})
	t.Run("an empty rules object accepts anything", func(t *testing.T) {
		ok(t, Rules{}, Data{"a": 1})
	})
	t.Run("an empty rule list checks nothing", func(t *testing.T) {
		ok(t, Rules{"f": []string{}}, Data{"f": 1})
	})
	t.Run("empty data satisfies an optional rule", func(t *testing.T) {
		ok(t, Rules{"a": "optional|string"}, Data{})
	})
	t.Run("a rule that is only optional", func(t *testing.T) {
		ok(t, Rules{"f": "optional"}, Data{})
		ok(t, Rules{"f": "optional"}, Data{"f": "anything"})
	})
}

// --- hostile paths --------------------------------------------------------

func TestHostilePaths(t *testing.T) {
	t.Run("a missing intermediate reports required", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{}, []string{"a.b.c is required"})
	})
	t.Run("a null intermediate does not panic", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": nil}, []string{"a.b.c is required"})
	})
	t.Run("an array intermediate reports that segment", func(t *testing.T) {
		fails(t, Rules{"a.b": "string"}, Data{"a": []any{1, 2}},
			[]string{"a must be of type object"})
	})
	t.Run("a number intermediate reports that segment", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": 5},
			[]string{"a must be of type object"})
	})
	t.Run("a string intermediate reports that segment", func(t *testing.T) {
		fails(t, Rules{"a.b": "string"}, Data{"a": "xy"},
			[]string{"a must be of type object"})
	})
	t.Run("a nested null mid-path reports required", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": Rules{"c": "string"}}}, Data{"a": Data{"b": nil}},
			[]string{"a.b is required"})
	})
	t.Run("indexing into an object reads as missing", func(t *testing.T) {
		fails(t, Rules{"a[0]": "string"}, Data{"a": Data{"x": 1}}, []string{"a[0] is required"})
	})
	t.Run("indexing into null reads as missing", func(t *testing.T) {
		fails(t, Rules{"a[0]": "string"}, Data{"a": nil}, []string{"a[0] is required"})
	})
	t.Run("indexing into a string reads as missing", func(t *testing.T) {
		fails(t, Rules{"a[0]": "string"}, Data{"a": "xy"}, []string{"a[0] is required"})
	})
	t.Run("a deeply nested missing leaf", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": Rules{"c": Rules{"d": "string"}}}},
			Data{"a": Data{"b": Data{"c": Data{}}}}, []string{"a.b.c.d is required"})
	})
	t.Run("a very deep path does not panic", func(t *testing.T) {
		key := strings.Repeat("a.", 50) + "leaf"
		fails(t, Rules{key: "string"}, Data{}, []string{key + " is required"})
	})
}

// --- numeric boundaries ---------------------------------------------------

func TestNumericBoundaries(t *testing.T) {
	t.Run("zero", func(t *testing.T) {
		fails(t, Rules{"f": "positive"}, Data{"f": 0}, []string{"f must be a valid positive number"})
		fails(t, Rules{"f": "negative"}, Data{"f": 0}, []string{"f must be a valid negative number"})
		fails(t, Rules{"f": "natural"}, Data{"f": 0}, []string{"f must be a valid natural number"})
		ok(t, Rules{"f": "whole"}, Data{"f": 0})
		ok(t, Rules{"f": "int"}, Data{"f": 0})
	})
	t.Run("negative zero is not natural", func(t *testing.T) {
		fails(t, Rules{"f": "natural"}, Data{"f": -0.0}, []string{"f must be a valid natural number"})
	})
	t.Run("a float with no fractional part is an integer", func(t *testing.T) {
		ok(t, Rules{"f": "int"}, Data{"f": 5.0})
		ok(t, Rules{"f": "natural"}, Data{"f": 5.0})
	})
	t.Run("NaN is a number but not a natural", func(t *testing.T) {
		// NaN is not an integer value, so it fails the integer rules.
		ok(t, Rules{"f": "number"}, Data{"f": math.NaN()})
		fails(t, Rules{"f": "natural"}, Data{"f": math.NaN()},
			[]string{"f must be a valid natural number"})
	})
	t.Run("NaN fails a decimal rule with its own code", func(t *testing.T) {
		fails(t, Rules{"f": "number|decimalsize:2"}, Data{"f": math.NaN()},
			[]string{"f must be a value number"})
		if got := singleCode(t, Rules{"f": "number|decimalsize:2"}, Data{"f": math.NaN()}); got != CodeNotANumber {
			t.Fatalf("expected NOT_A_NUMBER, got %s", got)
		}
	})
	t.Run("infinity is a number but not a natural", func(t *testing.T) {
		// Infinity is not an integer value either.
		ok(t, Rules{"f": "number"}, Data{"f": math.Inf(1)})
		ok(t, Rules{"f": "number"}, Data{"f": math.Inf(-1)})
		fails(t, Rules{"f": "natural"}, Data{"f": math.Inf(1)},
			[]string{"f must be a valid natural number"})
	})
	t.Run("bounds at the boundary are inclusive", func(t *testing.T) {
		ok(t, Rules{"f": "number|min:5"}, Data{"f": 5})
		ok(t, Rules{"f": "number|max:5"}, Data{"f": 5})
		fails(t, Rules{"f": "number|min:5"}, Data{"f": 4.999}, []string{"f must be at least 5"})
		fails(t, Rules{"f": "number|max:5"}, Data{"f": 5.001}, []string{"f must be at most 5"})
	})
	t.Run("negative bounds", func(t *testing.T) {
		ok(t, Rules{"f": "number|min:-10|max:-1"}, Data{"f": -5})
		fails(t, Rules{"f": "number|min:-10"}, Data{"f": -20}, []string{"f must be at least -10"})
	})
	t.Run("a very large float still counts digits", func(t *testing.T) {
		fails(t, Rules{"f": "number|size:3"}, Data{"f": 1e21}, []string{"f must have 3 digits"})
	})
	t.Run("an integer beyond float64 precision keeps its digits", func(t *testing.T) {
		huge, _ := new(big.Int).SetString("12345678901234567890", 10)
		ok(t, Rules{"f": "bigint"}, Data{"f": huge})
	})
	t.Run("json.Number satisfies numeric rules", func(t *testing.T) {
		dec := json.NewDecoder(strings.NewReader(`{"f": 42}`))
		dec.UseNumber()
		var d Data
		if err := dec.Decode(&d); err != nil {
			t.Fatal(err)
		}
		ok(t, Rules{"f": "natural|min:42|max:42"}, d)
	})
}

// --- string boundaries ----------------------------------------------------

func TestStringBoundaries(t *testing.T) {
	t.Run("an empty string is a string", func(t *testing.T) {
		ok(t, Rules{"f": "string"}, Data{"f": ""})
		ok(t, Rules{"f": "string|min:0"}, Data{"f": ""})
		ok(t, Rules{"f": "string|size:0"}, Data{"f": ""})
	})
	t.Run("an empty string fails a non-zero minimum", func(t *testing.T) {
		fails(t, Rules{"f": "string|min:1"}, Data{"f": ""},
			[]string{"f must have length of at least 1"})
	})
	t.Run("length counts runes, not bytes", func(t *testing.T) {
		ok(t, Rules{"f": "string|size:2"}, Data{"f": "日本"})
		ok(t, Rules{"f": "string|size:3"}, Data{"f": "héllo"[0:0] + "abc"})
	})
	t.Run("an emoji counts as one rune", func(t *testing.T) {
		// Lengths count runes, so an astral-plane character is one.
		ok(t, Rules{"f": "string|size:1"}, Data{"f": "👍"})
		fails(t, Rules{"f": "string|size:2"}, Data{"f": "👍"}, []string{"f must have length 2"})
	})
	t.Run("a combining mark counts separately", func(t *testing.T) {
		ok(t, Rules{"f": "string|size:2"}, Data{"f": "é"})
	})
	t.Run("a long string does not stall the format rules", func(t *testing.T) {
		long := strings.Repeat("a", 10000)
		fails(t, Rules{"f": "email"}, Data{"f": long}, []string{"f must be a valid email"})
		fails(t, Rules{"f": "name"}, Data{"f": long + "1"}, []string{"f must be a valid name"})
		ok(t, Rules{"f": "alpha"}, Data{"f": long})
	})
}

// --- array boundaries -----------------------------------------------------

func TestArrayBoundaries(t *testing.T) {
	t.Run("an empty array", func(t *testing.T) {
		ok(t, Rules{"f": "array"}, Data{"f": []any{}})
		ok(t, Rules{"f": "array|min:0"}, Data{"f": []any{}})
		ok(t, Rules{"f": "array|size:0"}, Data{"f": []any{}})
		ok(t, Rules{"f": "arrayof:string"}, Data{"f": []any{}})
		fails(t, Rules{"f": "array|min:1"}, Data{"f": []any{}},
			[]string{"f must have length of at least 1"})
	})
	t.Run("a string is not an array", func(t *testing.T) {
		fails(t, Rules{"f": "array"}, Data{"f": "ab"}, []string{"f must be an array"})
	})
	t.Run("an array is not an object", func(t *testing.T) {
		fails(t, Rules{"f": "object"}, Data{"f": []any{}}, []string{"f must be an object"})
	})
	t.Run("an object is not an array", func(t *testing.T) {
		fails(t, Rules{"f": "array"}, Data{"f": Data{}}, []string{"f must be an array"})
	})
	t.Run("a heterogeneous array reports each failure", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:number"}, Data{"f": []any{1, "x", 3, "y"}},
			[]string{"f[1] must be a valid number", "f[3] must be a valid number"})
	})
	t.Run("a large array is handled", func(t *testing.T) {
		big := make([]any, 1000)
		for i := range big {
			big[i] = i
		}
		ok(t, Rules{"f": "arrayof:whole"}, Data{"f": big})
	})
	t.Run("deeply nested arrays", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:arrayof:arrayof:arrayof:number"},
			Data{"f": []any{[]any{[]any{[]any{"x"}}}}},
			[]string{"f[0][0][0][0] must be a valid number"})
	})
}

// --- data guards ----------------------------------------------------------

func TestDataGuards(t *testing.T) {
	t.Run("nil data reports the data itself", func(t *testing.T) {
		result, err := Validate(Rules{"a": "string"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Details) != 1 || result.Details[0].Code != CodeDataRequired {
			t.Fatalf("expected DATA_REQUIRED, got %#v", result.Details)
		}
	})
	t.Run("empty data against required rules", func(t *testing.T) {
		fails(t, Rules{"a": "string", "b": "number"}, Data{},
			[]string{"a is required", "b is required"})
	})
	t.Run("data with extra keys passes without strict", func(t *testing.T) {
		ok(t, Rules{"a": "string"}, Data{"a": "x", "b": 1, "c": 2})
	})
	t.Run("a nil value inside a nested object", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": "string"}}, Data{"a": Data{"b": nil}},
			[]string{"a.b is required"})
	})
}

// --- rule and data interaction --------------------------------------------

func TestMixedStructures(t *testing.T) {
	t.Run("every structure kind in one rule set", func(t *testing.T) {
		rules := Rules{
			"scalar": "string",
			"nested": Rules{"inner": "natural"},
			"tuple":  []Rules{{"n": "email"}},
			"grid":   []any{[]Rules{{"label": "string"}}},
			"list":   "arrayof:number",
			"op":     Rules{"$or": []any{"email", "phone"}},
			"idx[0]": "boolean",
			"custom": CustomRule(func(v, _ any) *RuleError {
				if v == "ok" {
					return nil
				}
				return &RuleError{Message: "custom failed", Code: "C"}
			}),
		}
		data := Data{
			"scalar": "x",
			"nested": Data{"inner": 1},
			"tuple":  []any{Data{"n": "a@b.com"}},
			"grid":   []any{[]any{Data{"label": "L"}}},
			"list":   []any{1, 2},
			"op":     "a@b.com",
			"idx":    []any{true},
			"custom": "ok",
		}
		ok(t, rules, data)
	})
	t.Run("failures from every structure kind at once", func(t *testing.T) {
		rules := Rules{
			"scalar": "string",
			"nested": Rules{"inner": "natural"},
			"tuple":  []Rules{{"n": "email"}},
			"list":   "arrayof:number",
		}
		data := Data{
			"scalar": 1,
			"nested": Data{"inner": -1},
			"tuple":  []any{Data{"n": "bad"}},
			"list":   []any{"x"},
		}
		fails(t, rules, data, []string{
			"scalar must be string",
			"nested.inner must be a valid natural number",
			"tuple[0].n must be a valid email",
			"list[0] must be a valid number",
		})
	})
}
