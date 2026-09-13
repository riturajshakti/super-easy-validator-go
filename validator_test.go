package validator

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

func errorsOf(t *testing.T, rules Rules, data Data, config ...Config) []string {
	t.Helper()
	result, err := Validate(rules, data, config...)
	if err != nil {
		t.Fatalf("unexpected rule error: %v", err)
	}
	return result.Errors
}

func ok(t *testing.T, rules Rules, data Data, config ...Config) {
	t.Helper()
	if errs := errorsOf(t, rules, data, config...); errs != nil {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

// fails compares without assuming order, since error order within one rules
// level is unspecified: Go randomizes map iteration.
func fails(t *testing.T, rules Rules, data Data, expected []string, config ...Config) {
	t.Helper()
	got := errorsOf(t, rules, data, config...)
	if !sameSet(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func sameSet(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	used := make([]bool, len(want))
	for _, g := range got {
		found := false
		for i, w := range want {
			if !used[i] && g == w {
				used[i], found = true, true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func throws(t *testing.T, rules Rules, data Data) {
	t.Helper()
	if _, err := Validate(rules, data); err == nil {
		t.Fatal("expected an InvalidRuleError, got nil")
	} else if _, isRuleErr := err.(*InvalidRuleError); !isRuleErr {
		t.Fatalf("expected *InvalidRuleError, got %T", err)
	}
}

func codesOf(t *testing.T, rules Rules, data Data) []ErrorCode {
	t.Helper()
	result, err := Validate(rules, data)
	if err != nil {
		t.Fatalf("unexpected rule error: %v", err)
	}
	out := make([]ErrorCode, len(result.Details))
	for i, d := range result.Details {
		out[i] = d.Code
	}
	return out
}

// --- presence -------------------------------------------------------------

func TestOptionalAndNullable(t *testing.T) {
	t.Run("optional allows an absent field", func(t *testing.T) {
		ok(t, Rules{"f": "optional|string"}, Data{})
	})
	t.Run("optional still validates a present field", func(t *testing.T) {
		fails(t, Rules{"f": "optional|string"}, Data{"f": 1}, []string{"f must be string"})
	})
	t.Run("optional does not allow null", func(t *testing.T) {
		fails(t, Rules{"f": "optional|string"}, Data{"f": nil}, []string{"f is required"})
	})
	t.Run("nullable allows null", func(t *testing.T) {
		ok(t, Rules{"f": "nullable|string"}, Data{"f": nil})
	})
	t.Run("nullable does not allow absent", func(t *testing.T) {
		fails(t, Rules{"f": "nullable|string"}, Data{}, []string{"f is required"})
	})
	t.Run("optional|nullable allows both", func(t *testing.T) {
		ok(t, Rules{"f": "optional|nullable|string"}, Data{})
		ok(t, Rules{"f": "optional|nullable|string"}, Data{"f": nil})
	})
}

func TestPresenceIsNotTruthiness(t *testing.T) {
	rules := Rules{"users": []Rules{{"name": "string"}}}
	t.Run("empty string is a type error, not required", func(t *testing.T) {
		fails(t, rules, Data{"users": ""}, []string{"users must be of type array"})
	})
	t.Run("zero is a type error, not required", func(t *testing.T) {
		fails(t, rules, Data{"users": 0}, []string{"users must be of type array"})
	})
	t.Run("false is a type error, not required", func(t *testing.T) {
		fails(t, rules, Data{"users": false}, []string{"users must be of type array"})
	})
	t.Run("null reports required", func(t *testing.T) {
		fails(t, rules, Data{"users": nil}, []string{"users is required"})
	})
}

// --- data types -----------------------------------------------------------

func TestDataTypes(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		ok(t, Rules{"f": "string"}, Data{"f": "a"})
		fails(t, Rules{"f": "string"}, Data{"f": 1}, []string{"f must be string"})
	})
	t.Run("number rejects a numeric string", func(t *testing.T) {
		fails(t, Rules{"f": "number"}, Data{"f": "42"}, []string{"f must be a valid number"})
	})
	t.Run("string|number accepts a numeric string", func(t *testing.T) {
		ok(t, Rules{"f": "string|number"}, Data{"f": "42"})
	})
	t.Run("string|number rejects a non-numeric string", func(t *testing.T) {
		fails(t, Rules{"f": "string|number"}, Data{"f": "abc"},
			[]string{"f must be a valid numeric string"})
	})
	t.Run("boolean", func(t *testing.T) {
		ok(t, Rules{"f": "boolean"}, Data{"f": true})
		fails(t, Rules{"f": "boolean"}, Data{"f": "true"}, []string{"f must be a valid boolean"})
	})
	t.Run("string|boolean accepts a boolean string", func(t *testing.T) {
		ok(t, Rules{"f": "string|boolean"}, Data{"f": "true"})
		fails(t, Rules{"f": "string|boolean"}, Data{"f": "yes"},
			[]string{"f must be a valid boolean string"})
	})
	t.Run("array", func(t *testing.T) {
		ok(t, Rules{"f": "array"}, Data{"f": []any{1}})
		fails(t, Rules{"f": "array"}, Data{"f": "ab"}, []string{"f must be an array"})
	})
	t.Run("object", func(t *testing.T) {
		ok(t, Rules{"f": "object"}, Data{"f": map[string]any{}})
		fails(t, Rules{"f": "object"}, Data{"f": []any{}}, []string{"f must be an object"})
	})
	t.Run("native Go slices are arrays", func(t *testing.T) {
		ok(t, Rules{"f": "array"}, Data{"f": []string{"a", "b"}})
		ok(t, Rules{"f": "arrayof:string"}, Data{"f": []string{"a", "b"}})
	})
}

// --- numbers --------------------------------------------------------------

func TestNumberTypes(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		ok(t, Rules{"f": "int"}, Data{"f": 5})
		fails(t, Rules{"f": "int"}, Data{"f": 5.5}, []string{"f must be a valid integer"})
	})
	t.Run("positive", func(t *testing.T) {
		ok(t, Rules{"f": "positive"}, Data{"f": 1})
		fails(t, Rules{"f": "positive"}, Data{"f": 0}, []string{"f must be a valid positive number"})
	})
	t.Run("negative", func(t *testing.T) {
		ok(t, Rules{"f": "negative"}, Data{"f": -1})
		fails(t, Rules{"f": "negative"}, Data{"f": 0}, []string{"f must be a valid negative number"})
	})
	t.Run("natural", func(t *testing.T) {
		ok(t, Rules{"f": "natural"}, Data{"f": 1})
		fails(t, Rules{"f": "natural"}, Data{"f": 0}, []string{"f must be a valid natural number"})
		fails(t, Rules{"f": "natural"}, Data{"f": 4.5}, []string{"f must be a valid natural number"})
	})
	t.Run("whole", func(t *testing.T) {
		ok(t, Rules{"f": "whole"}, Data{"f": 0})
		fails(t, Rules{"f": "whole"}, Data{"f": -1}, []string{"f must be a valid whole number"})
	})
	t.Run("numeric string variants use their own message", func(t *testing.T) {
		ok(t, Rules{"f": "string|natural"}, Data{"f": "5"})
		fails(t, Rules{"f": "string|natural"}, Data{"f": "0"},
			[]string{"f must be a valid natural numeric string"})
		fails(t, Rules{"f": "string|positive"}, Data{"f": "-1"},
			[]string{"f must be a valid positive numeric string"})
	})
}

// Every Go numeric kind normalizes to one int and one float, so callers never
// reason about bit widths.
func TestNumericKindsNormalize(t *testing.T) {
	for _, v := range []any{int(5), int8(5), int16(5), int32(5), int64(5),
		uint(5), uint8(5), uint16(5), uint32(5), uint64(5), float32(5), float64(5)} {
		ok(t, Rules{"f": "natural|min:5|max:5"}, Data{"f": v})
	}
}

func TestIntegerRulesFollowValueNotType(t *testing.T) {
	t.Run("a whole-valued float satisfies natural", func(t *testing.T) {
		ok(t, Rules{"f": "natural"}, Data{"f": float64(7)})
	})
	t.Run("a fractional float does not", func(t *testing.T) {
		fails(t, Rules{"f": "natural"}, Data{"f": float64(7.5)},
			[]string{"f must be a valid natural number"})
	})
	t.Run("JSON-decoded integers satisfy natural", func(t *testing.T) {
		var d Data
		if err := json.Unmarshal([]byte(`{"f": 7}`), &d); err != nil {
			t.Fatal(err)
		}
		ok(t, Rules{"f": "natural"}, d)
	})
}

func TestBigint(t *testing.T) {
	t.Run("accepts a big.Int", func(t *testing.T) {
		ok(t, Rules{"f": "bigint"}, Data{"f": big.NewInt(42)})
	})
	t.Run("rejects a plain number", func(t *testing.T) {
		fails(t, Rules{"f": "bigint"}, Data{"f": 42}, []string{"f must be bigint"})
	})
	t.Run("a value beyond float64 survives exactly", func(t *testing.T) {
		huge, _ := new(big.Int).SetString("12345678901234567890", 10)
		ok(t, Rules{"f": "bigint"}, Data{"f": huge})
		if huge.String() != "12345678901234567890" {
			t.Fatalf("precision lost: %s", huge)
		}
	})
	t.Run("accepts json.Number from a UseNumber decode", func(t *testing.T) {
		dec := json.NewDecoder(strings.NewReader(`{"f": 12345678901234567890}`))
		dec.UseNumber()
		var d Data
		if err := dec.Decode(&d); err != nil {
			t.Fatal(err)
		}
		ok(t, Rules{"f": "bigint"}, d)
	})
}

// --- string formats -------------------------------------------------------

func TestStringFormats(t *testing.T) {
	cases := []struct {
		rule  string
		good  []string
		bad   []string
		error string
	}{
		{"email", []string{"a@b.com", "first.last@sub.domain.org"},
			[]string{"not-an-email", "a@", "@b.com"}, "f must be a valid email"},
		{"url", []string{"https://example.com", "http://www.example.com", "www.example.com", "https://sub.example.co.uk"},
			[]string{"example.com", "not a url"}, "f must be a valid url"},
		{"username", []string{"johndoe123", "john.doe.x"},
			[]string{"john..doe", "john._doe", "john__doe", "_johndoe", "johndoe_", "short"},
			"f must be a valid username"},
		{"alpha", []string{"abcDEF"}, []string{"abc1", ""}, "f must be a valid alpha"},
		{"alphanumeric", []string{"abc123"}, []string{"abc-123"}, "f must be a valid alphanumeric"},
		{"phonecode", []string{"+1", "+91"}, []string{"91", "+1234"}, "f must be a valid phone code"},
		{"objectid", []string{"507f1f77bcf86cd799439011"}, []string{"abc", "507f1f77bcf86cd79943901"},
			"f must be a valid object id"},
		{"uuid", []string{"123e4567-e89b-12d3-a456-426655440000"}, []string{"123e4567"},
			"f must be a valid uuid"},
		{"dateonly", []string{"2024-06-01"}, []string{"2024-13-01", "01-06-2024"}, "f must be a valid date"},
		{"time", []string{"12:30", "23:59:59"}, []string{"24:00", "12:60"}, "f must be a valid time"},
		{"lower", []string{"abc-1"}, []string{"Abc"}, "f must not contains upper case letters"},
		{"upper", []string{"ABC-1"}, []string{"AbC"}, "f must not contains lower case letters"},
		{"ip", []string{"192.168.1.1", "0.0.0.0"}, []string{"256.1.1.1", "1.2.3"}, "f must be a valid IP address"},
	}

	for _, tc := range cases {
		t.Run(tc.rule, func(t *testing.T) {
			for _, g := range tc.good {
				ok(t, Rules{"f": tc.rule}, Data{"f": g})
			}
			for _, b := range tc.bad {
				fails(t, Rules{"f": tc.rule}, Data{"f": b}, []string{tc.error})
			}
		})
	}
}

func TestNameAndFullname(t *testing.T) {
	t.Run("name accepts diacritics and separators", func(t *testing.T) {
		for _, s := range []string{"John", "Rock Port", "José", "O'Brien", "Jean-Luc", "Müller"} {
			ok(t, Rules{"f": "name"}, Data{"f": s})
		}
	})
	t.Run("name rejects digits", func(t *testing.T) {
		fails(t, Rules{"f": "name"}, Data{"f": "123"}, []string{"f must be a valid name"})
	})
	t.Run("fullname needs at least two parts", func(t *testing.T) {
		ok(t, Rules{"f": "fullname"}, Data{"f": "John Doe"})
		fails(t, Rules{"f": "fullname"}, Data{"f": "John"}, []string{"f must be a valid fullname"})
	})
	t.Run("non-Latin scripts are accepted", func(t *testing.T) {
		ok(t, Rules{"f": "name"}, Data{"f": "राज"})
		ok(t, Rules{"f": "name"}, Data{"f": "北京"})
	})
}

// --- constraints ----------------------------------------------------------

func TestConstraints(t *testing.T) {
	t.Run("equal auto-detects the value type", func(t *testing.T) {
		ok(t, Rules{"f": "equal:200"}, Data{"f": "200"})
		ok(t, Rules{"f": "equal:200"}, Data{"f": 200})
		fails(t, Rules{"f": "equal:200"}, Data{"f": 201}, []string{"f must be equal to 200"})
	})
	t.Run("size on a string is length", func(t *testing.T) {
		ok(t, Rules{"f": "string|size:3"}, Data{"f": "abc"})
		fails(t, Rules{"f": "string|size:3"}, Data{"f": "ab"}, []string{"f must have length 3"})
	})
	t.Run("size on an array is length", func(t *testing.T) {
		ok(t, Rules{"f": "array|size:2"}, Data{"f": []any{1, 2}})
	})
	t.Run("size on a number counts digits, ignoring sign and point", func(t *testing.T) {
		ok(t, Rules{"f": "number|size:3"}, Data{"f": 123})
		ok(t, Rules{"f": "number|size:3"}, Data{"f": -123})
		ok(t, Rules{"f": "number|size:3"}, Data{"f": 1.23})
		fails(t, Rules{"f": "number|size:3"}, Data{"f": 12}, []string{"f must have 3 digits"})
	})
	t.Run("min and max on string length", func(t *testing.T) {
		fails(t, Rules{"f": "string|min:5"}, Data{"f": "ab"},
			[]string{"f must have length of at least 5"})
		fails(t, Rules{"f": "string|max:2"}, Data{"f": "abc"},
			[]string{"f must have length of at most 2"})
	})
	t.Run("min and max on numbers", func(t *testing.T) {
		fails(t, Rules{"f": "number|min:18"}, Data{"f": 15}, []string{"f must be at least 18"})
		fails(t, Rules{"f": "number|max:90"}, Data{"f": 200}, []string{"f must be at most 90"})
	})
	t.Run("min and max on array length", func(t *testing.T) {
		fails(t, Rules{"f": "array|min:3"}, Data{"f": []any{1}},
			[]string{"f must have length of at least 3"})
	})
	t.Run("enums", func(t *testing.T) {
		ok(t, Rules{"f": "enums:admin,user"}, Data{"f": "admin"})
		fails(t, Rules{"f": "enums:admin,user"}, Data{"f": "root"}, []string{"f is invalid"})
	})
	t.Run("numeric enums", func(t *testing.T) {
		ok(t, Rules{"f": "number|enums:1,2"}, Data{"f": 1})
		fails(t, Rules{"f": "number|enums:1,2"}, Data{"f": 3}, []string{"f is invalid"})
	})
}

func TestDecimals(t *testing.T) {
	t.Run("decimalsize", func(t *testing.T) {
		ok(t, Rules{"f": "number|decimalsize:2"}, Data{"f": 1.25})
		fails(t, Rules{"f": "number|decimalsize:2"}, Data{"f": 1.2},
			[]string{"f must have 2 decimal places"})
	})
	t.Run("a numeric string keeps trailing zeroes", func(t *testing.T) {
		ok(t, Rules{"f": "string|number|decimalsize:2"}, Data{"f": "10.50"})
	})
	t.Run("decimalmin and decimalmax", func(t *testing.T) {
		ok(t, Rules{"f": "number|decimalmin:1"}, Data{"f": 1.5})
		fails(t, Rules{"f": "number|decimalmax:2"}, Data{"f": 5000.123},
			[]string{"f must have at most 2 decimal places"})
	})
}

// min: and max: compare dates when the argument is not numeric.
func TestDateBounds(t *testing.T) {
	t.Run("min rejects an earlier date", func(t *testing.T) {
		fails(t, Rules{"d": "date|min:2024-01-01"}, Data{"d": "2023-06-01"},
			[]string{"d must be at least 2024-01-01T00:00:00.000Z"})
	})
	t.Run("min accepts a later date", func(t *testing.T) {
		ok(t, Rules{"d": "date|min:2024-01-01"}, Data{"d": "2024-06-01"})
	})
	t.Run("max rejects a later date", func(t *testing.T) {
		fails(t, Rules{"d": "date|max:2024-12-31"}, Data{"d": "2025-06-01"},
			[]string{"d must be at most 2024-12-31T00:00:00.000Z"})
	})
	t.Run("max accepts an earlier date", func(t *testing.T) {
		ok(t, Rules{"d": "date|max:2024-12-31"}, Data{"d": "2024-06-01"})
	})
	t.Run("dateonly works with bounds", func(t *testing.T) {
		fails(t, Rules{"d": "dateonly|min:2024-01-01"}, Data{"d": "2023-01-01"},
			[]string{"d must be at least 2024-01-01T00:00:00.000Z"})
	})
	t.Run("the codes are date-specific", func(t *testing.T) {
		if got := codesOf(t, Rules{"d": "date|min:2024-01-01"}, Data{"d": "2023-06-01"}); len(got) != 1 || got[0] != CodeDateTooEarly {
			t.Fatalf("expected DATE_TOO_EARLY, got %v", got)
		}
		if got := codesOf(t, Rules{"d": "date|max:2024-01-01"}, Data{"d": "2025-06-01"}); len(got) != 1 || got[0] != CodeDateTooLate {
			t.Fatalf("expected DATE_TOO_LATE, got %v", got)
		}
	})
	t.Run("a full timestamp compares correctly", func(t *testing.T) {
		ok(t, Rules{"d": "date|min:2024-01-01"}, Data{"d": "2024-06-01T12:30:00Z"})
	})
}

func TestMustValidate(t *testing.T) {
	t.Run("returns the result for valid rules", func(t *testing.T) {
		result := MustValidate(Rules{"a": "string"}, Data{"a": 1})
		if len(result.Errors) != 1 || result.Errors[0] != "a must be string" {
			t.Fatalf("unexpected result: %#v", result)
		}
	})
	t.Run("panics on a malformed rule", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("expected a panic for a malformed rule")
			}
		}()
		MustValidate(Rules{"a": "naturl"}, Data{"a": 1})
	})
}

func TestRegexRule(t *testing.T) {
	t.Run("JS literal syntax is accepted", func(t *testing.T) {
		ok(t, Rules{"f": `regex:/^[A-Z0-9]{4}$/i`}, Data{"f": "ab12"})
		fails(t, Rules{"f": `regex:/^[A-Z0-9]{4}$/i`}, Data{"f": "ab1"}, []string{"f is invalid"})
	})
	t.Run("Go inline-flag syntax is accepted", func(t *testing.T) {
		ok(t, Rules{"f": `regex:(?i)^[A-Z0-9]{4}$`}, Data{"f": "AB12"})
	})
	t.Run("iteration flags are ignored", func(t *testing.T) {
		ok(t, Rules{"f": `regex:/^x$/gu`}, Data{"f": "x"})
	})
	t.Run("a pattern containing a pipe needs the list form", func(t *testing.T) {
		ok(t, Rules{"f": []string{"string", `regex:/^(male)|(female)$/`}}, Data{"f": "male"})
	})
	t.Run("lookahead is rejected as a rule error", func(t *testing.T) {
		throws(t, Rules{"f": `regex:/^(?!www)x$/`}, Data{"f": "x"})
	})
	t.Run("a backreference is rejected as a rule error", func(t *testing.T) {
		throws(t, Rules{"f": `regex:/^(a)\1$/`}, Data{"f": "aa"})
	})
}

// --- arrayof --------------------------------------------------------------

func TestArrayOf(t *testing.T) {
	t.Run("each element is a string", func(t *testing.T) {
		ok(t, Rules{"f": "arrayof:string"}, Data{"f": []any{"a", "b"}})
	})
	t.Run("reports the offending index", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:string"}, Data{"f": []any{"a", 1}},
			[]string{"f[1] must be string"})
	})
	t.Run("applies a constraint per element", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:max:10"},
			Data{"f": []any{"ok", "this one is far too long"}},
			[]string{"f[1] must have length of at most 10"})
	})
	t.Run("rejects a non-array", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:string"}, Data{"f": "ab"}, []string{"f must be an array"})
	})
	t.Run("combines with array-level constraints", func(t *testing.T) {
		fails(t, Rules{"f": "array|min:3|arrayof:string"}, Data{"f": []any{"a"}},
			[]string{"f must have length of at least 3"})
	})
	t.Run("arrayof:optional skips nulls", func(t *testing.T) {
		ok(t, Rules{"f": "arrayof:optional|arrayof:email"}, Data{"f": []any{"a@b.com", nil}})
	})
	t.Run("numeric string elements", func(t *testing.T) {
		ok(t, Rules{"f": "arrayof:string|arrayof:natural"}, Data{"f": []any{"1", "2"}})
	})
	t.Run("nested arrayof labels every depth", func(t *testing.T) {
		fails(t, Rules{"m": "arrayof:arrayof:number"},
			Data{"m": []any{[]any{1}, []any{"x"}}},
			[]string{"m[1][0] must be a valid number"})
	})
	t.Run("heterogeneous arrays report each failure", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:email"},
			Data{"f": []any{"bad", "a@b.com", "worse"}},
			[]string{"f[0] must be a valid email", "f[2] must be a valid email"})
	})
}

// --- structure ------------------------------------------------------------

func TestNestedObjects(t *testing.T) {
	rules := Rules{"address": Rules{"city": "name", "country": Rules{"code": "alpha|upper|size:2"}}}

	t.Run("accepts a valid nested object", func(t *testing.T) {
		ok(t, rules, Data{"address": Data{"city": "Rock Port", "country": Data{"code": "IN"}}})
	})
	t.Run("labels errors with a dotted path", func(t *testing.T) {
		fails(t, rules, Data{"address": Data{"city": "Rock Port", "country": Data{"code": "in"}}},
			[]string{"address.country.code must not contains lower case letters"})
	})
	t.Run("reports a missing deep field", func(t *testing.T) {
		fails(t, rules, Data{"address": Data{"city": "Rock Port", "country": Data{}}},
			[]string{"address.country.code is required"})
	})
	t.Run("null nested object reports required", func(t *testing.T) {
		fails(t, rules, Data{"address": nil}, []string{"address is required"})
	})
	t.Run("absent nested object reports required", func(t *testing.T) {
		fails(t, rules, Data{}, []string{"address is required"})
	})
	t.Run("non-object reports a type error", func(t *testing.T) {
		fails(t, rules, Data{"address": "x"}, []string{"address must be of type object"})
	})
	t.Run("plain map[string]any works as nested data", func(t *testing.T) {
		ok(t, rules, Data{"address": map[string]any{
			"city": "Rock Port", "country": map[string]any{"code": "IN"}}})
	})
}

func TestArraysOfObjects(t *testing.T) {
	rules := Rules{"users": []Rules{{"name": "name", "age": "natural"}}}

	t.Run("accepts valid elements", func(t *testing.T) {
		ok(t, rules, Data{"users": []any{Data{"name": "Jo", "age": 20}}})
	})
	t.Run("reports per-element paths", func(t *testing.T) {
		fails(t, rules, Data{"users": []any{Data{"name": "Jo", "age": 20}, Data{}}},
			[]string{"users[1].name is required", "users[1].age is required"})
	})
	t.Run("rejects a non-array", func(t *testing.T) {
		fails(t, rules, Data{"users": "x"}, []string{"users must be of type array"})
	})
}

func TestDotNotationKeys(t *testing.T) {
	t.Run("dotted keys resolve", func(t *testing.T) {
		ok(t, Rules{"a.b": "string"}, Data{"a": Data{"b": "x"}})
	})
	t.Run("a null intermediate does not crash", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": nil}, []string{"a.b.c is required"})
	})
	t.Run("an absent intermediate reports required", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{}, []string{"a.b.c is required"})
	})
	t.Run("a non-object intermediate reports that segment", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": 5}, []string{"a must be of type object"})
	})
}

// A dotted key asserts that every intermediate segment is an object. When one
// is not, the segment that is wrong is reported rather than the unreachable
// leaf.
func TestDottedChainAssertsObjects(t *testing.T) {
	t.Run("the first segment breaks", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": 5},
			[]string{"a must be of type object"})
	})
	t.Run("a middle segment breaks", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": Data{"b": 5}},
			[]string{"a.b must be of type object"})
	})
	t.Run("a deep segment breaks", func(t *testing.T) {
		fails(t, Rules{"a.b.c.d": "string"}, Data{"a": Data{"b": Data{"c": "x"}}},
			[]string{"a.b.c must be of type object"})
	})
	t.Run("every non-object kind breaks the chain", func(t *testing.T) {
		for _, v := range []any{5, "xy", true, []any{1}, 1.5} {
			fails(t, Rules{"a.b": "string"}, Data{"a": v},
				[]string{"a must be of type object"})
		}
	})
	t.Run("the code is NOT_OBJECT", func(t *testing.T) {
		if got := singleCode(t, Rules{"a.b.c": "string"}, Data{"a": 5}); got != CodeNotObject {
			t.Fatalf("expected NOT_OBJECT, got %s", got)
		}
	})
	t.Run("optional does not excuse a present non-object", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "optional|string"}, Data{"a": 5},
			[]string{"a must be of type object"})
	})
	t.Run("optional still excuses an absent path", func(t *testing.T) {
		ok(t, Rules{"a.b.c": "optional|string"}, Data{})
		ok(t, Rules{"a.b.c": "optional|string"}, Data{"a": Data{"b": Data{}}})
	})
	t.Run("a nil intermediate counts as absent, not broken", func(t *testing.T) {
		fails(t, Rules{"a.b.c": "string"}, Data{"a": nil}, []string{"a.b.c is required"})
		ok(t, Rules{"a.b.c": "optional|string"}, Data{"a": nil})
	})
	t.Run("a walkable chain still validates the leaf", func(t *testing.T) {
		ok(t, Rules{"a.b.c": "string"}, Data{"a": Data{"b": Data{"c": "x"}}})
		fails(t, Rules{"a.b.c": "string"}, Data{"a": Data{"b": Data{"c": 1}}},
			[]string{"a.b.c must be string"})
	})
	t.Run("it matches the equivalent nested rule", func(t *testing.T) {
		nested := errorsOf(t, Rules{"a": Rules{"b": "string"}}, Data{"a": 5})
		dotted := errorsOf(t, Rules{"a.b": "string"}, Data{"a": 5})
		if len(nested) != 1 || len(dotted) != 1 || nested[0] != dotted[0] {
			t.Fatalf("nested %v and dotted %v should agree", nested, dotted)
		}
	})
	t.Run("an operator on a broken chain reports the segment", func(t *testing.T) {
		fails(t, Rules{"a.b": Rules{"$or": []any{"string", "number"}}}, Data{"a": 5},
			[]string{"a must be of type object"})
	})
	t.Run("a group key counts a broken chain as absent", func(t *testing.T) {
		fails(t, Rules{"x": "optional|string", "$atleast": "x|a.b"}, Data{"a": 5},
			[]string{"at least one of x and a.b is required"})
	})
}

// The reported segment is labelled in whatever scope the rule sits in.
func TestBrokenChainScoping(t *testing.T) {
	t.Run("inside a nested object rule", func(t *testing.T) {
		fails(t, Rules{"o": Rules{"a.b": "string"}}, Data{"o": Data{"a": 5}},
			[]string{"o.a must be of type object"})
	})
	t.Run("two levels of nesting", func(t *testing.T) {
		fails(t, Rules{"x": Rules{"o": Rules{"a.b": "string"}}}, Data{"x": Data{"o": Data{"a": 5}}},
			[]string{"x.o.a must be of type object"})
	})
	t.Run("inside a tuple element", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"a.b": "string"}}}, Data{"u": []any{Data{"a": 5}}},
			[]string{"u[0].a must be of type object"})
	})
	t.Run("an operator inside a nested object rule", func(t *testing.T) {
		fails(t, Rules{"o": Rules{"a.b": Rules{"$or": []any{"string"}}}}, Data{"o": Data{"a": 5}},
			[]string{"o.a must be of type object"})
	})
	t.Run("an operator inside a tuple element", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"a.b": Rules{"$or": []any{"string"}}}}},
			Data{"u": []any{Data{"a": 5}}}, []string{"u[0].a must be of type object"})
	})
	t.Run("a deep segment inside a nested object rule", func(t *testing.T) {
		fails(t, Rules{"o": Rules{"a.b.c": "string"}}, Data{"o": Data{"a": Data{"b": 5}}},
			[]string{"o.a.b must be of type object"})
	})
	t.Run("the quote style applies", func(t *testing.T) {
		fails(t, Rules{"a.b": "string"}, Data{"a": 5},
			[]string{"`a` must be of type object"}, Config{Quotes: QuoteTick})
		fails(t, Rules{"o": Rules{"a.b": "string"}}, Data{"o": Data{"a": 5}},
			[]string{`"o.a" must be of type object`}, Config{Quotes: QuoteDouble})
	})
	t.Run("a custom rule still owns its own decision", func(t *testing.T) {
		rules := Rules{"a.b": CustomRule(func(v, _ any) *RuleError {
			if v == nil {
				return &RuleError{Message: "custom saw nil", Code: "N"}
			}
			return &RuleError{Message: "custom saw a value", Code: "V"}
		})}
		fails(t, rules, Data{"a": 5}, []string{"custom saw nil"})
	})
	t.Run("strict also reports the undeclared base key", func(t *testing.T) {
		// A dotted rule key does not declare its base, so under strict the
		// base is an unexpected field as well as a broken chain.
		fails(t, Rules{"a.b": "string"}, Data{"a": 5},
			[]string{"a must be of type object", "a is not required"}, Config{Strict: true})
	})
}

// --- indexing -------------------------------------------------------------

func TestIndexing(t *testing.T) {
	t.Run("positive index", func(t *testing.T) {
		ok(t, Rules{"c[0]": "number"}, Data{"c": []any{1, 2, 3}})
		fails(t, Rules{"c[0]": "number"}, Data{"c": []any{"x", 2}},
			[]string{"c[0] must be a valid number"})
	})
	t.Run("several indexes on one array", func(t *testing.T) {
		fails(t, Rules{"c[0]": "number", "c[2]": "number"}, Data{"c": []any{"x", 2, "y"}},
			[]string{"c[0] must be a valid number", "c[2] must be a valid number"})
	})
	t.Run("out of range reads as missing", func(t *testing.T) {
		fails(t, Rules{"c[9]": "number"}, Data{"c": []any{1}}, []string{"c[9] is required"})
	})
	t.Run("index on a non-array reads as missing", func(t *testing.T) {
		fails(t, Rules{"c[0]": "number"}, Data{"c": "notarray"}, []string{"c[0] is required"})
	})
	t.Run("optional allows a missing index", func(t *testing.T) {
		ok(t, Rules{"c[9]": "optional|number"}, Data{"c": []any{1}})
	})
	t.Run("negative index", func(t *testing.T) {
		ok(t, Rules{"c[-1]": "number"}, Data{"c": []any{1, 2, 3}})
		fails(t, Rules{"c[-1]": "number"}, Data{"c": []any{1, "x"}},
			[]string{"c[-1] must be a valid number"})
		fails(t, Rules{"c[-9]": "number"}, Data{"c": []any{1}}, []string{"c[-9] is required"})
	})
	t.Run("slices apply the rule to each selected element", func(t *testing.T) {
		ok(t, Rules{"c[0:2]": "number"}, Data{"c": []any{1, 2, 3}})
		fails(t, Rules{"c[0:2]": "number"}, Data{"c": []any{1, "x", 3}},
			[]string{"c[1] must be a valid number"})
		fails(t, Rules{"c[0:2]": "number"}, Data{"c": []any{"a", "b", 3}},
			[]string{"c[0] must be a valid number", "c[1] must be a valid number"})
	})
	t.Run("elements outside the slice are ignored", func(t *testing.T) {
		ok(t, Rules{"c[0:2]": "number"}, Data{"c": []any{1, 2, "x"}})
	})
	t.Run("indexing can be disabled", func(t *testing.T) {
		ok(t, Rules{"c[0]": "optional|string"}, Data{"c": []any{1}},
			Config{DisableArrayIndexing: true})
	})
	t.Run("mixed object and index paths", func(t *testing.T) {
		fails(t, Rules{"order.items[0].sku": "string"},
			Data{"order": Data{"items": []any{Data{"sku": 1}}}},
			[]string{"order.items[0].sku must be string"})
	})
}

// --- operators ------------------------------------------------------------

func TestOr(t *testing.T) {
	t.Run("accepts either scalar branch", func(t *testing.T) {
		rules := Rules{"id": Rules{OpOr: []any{"objectid", "uuid"}}}
		ok(t, rules, Data{"id": "507f1f77bcf86cd799439011"})
		ok(t, rules, Data{"id": "123e4567-e89b-12d3-a456-426655440000"})
		if errs := errorsOf(t, rules, Data{"id": "nope"}); len(errs) == 0 {
			t.Fatal("expected an error for a value matching neither branch")
		}
	})
	t.Run("a single-branch Or behaves like the branch", func(t *testing.T) {
		fails(t, Rules{"a": Rules{OpOr: []any{"email"}}}, Data{"a": "bad"},
			[]string{"a must be a valid email"})
	})
	t.Run("object data reports the object branch", func(t *testing.T) {
		rules := Rules{"address": Rules{OpOr: []any{
			"string|max:20", Rules{"city": "name", "pin": "string|natural|size:6"}}}}
		ok(t, rules, Data{"address": "221B Baker Street"})
		ok(t, rules, Data{"address": Data{"city": "London", "pin": "123456"}})
		fails(t, rules, Data{"address": Data{"city": "123", "pin": "123456"}},
			[]string{"address.city must be a valid name"})
		fails(t, rules, Data{"address": Data{"city": "London"}},
			[]string{"address.pin is required"})
	})
	t.Run("type preference covers every scalar shape", func(t *testing.T) {
		rules := Rules{"v": Rules{OpOr: []any{"number|min:100", "boolean", "arrayof:number"}}}
		ok(t, rules, Data{"v": 500})
		ok(t, rules, Data{"v": true})
		ok(t, rules, Data{"v": []any{1, 2}})
		fails(t, rules, Data{"v": 5}, []string{"v must be at least 100"})
	})
	t.Run("a tuple branch matches array data", func(t *testing.T) {
		rules := Rules{"v": Rules{OpOr: []any{"string", []Rules{{"a": "natural"}}}}}
		ok(t, rules, Data{"v": []any{Data{"a": 1}}})
		fails(t, rules, Data{"v": []any{Data{"a": -1}}},
			[]string{"v[0].a must be a valid natural number"})
	})
	t.Run("a custom rule branch always applies", func(t *testing.T) {
		rules := Rules{"v": Rules{OpOr: []any{"string|min:10", CustomRule(func(v, _ any) *RuleError {
			if v == "shortcut" {
				return nil
			}
			return &RuleError{Message: "no shortcut", Code: "NO_SHORTCUT"}
		})}}}
		ok(t, rules, Data{"v": "shortcut"})
	})
	t.Run("type preference picks the intended branch", func(t *testing.T) {
		rules := Rules{"v": Rules{OpOr: []any{"string|min:10", Rules{"a": "natural"}}}}
		fails(t, rules, Data{"v": Data{"a": -1}}, []string{"v.a must be a valid natural number"})
		fails(t, rules, Data{"v": "short"}, []string{"v must have length of at least 10"})
	})
	t.Run("the branch with fewer failures wins", func(t *testing.T) {
		rules := Rules{"v": Rules{OpOr: []any{
			Rules{"a": "string", "b": "string", "c": "string"}, Rules{"a": "string"}}}}
		fails(t, rules, Data{"v": Data{"a": 1}}, []string{"v.a must be string"})
	})
	t.Run("an only-optional branch is rejected as a rule error", func(t *testing.T) {
		throws(t, Rules{"v": Rules{OpOr: []any{"optional", "string"}}}, Data{"v": 1})
	})
}

func TestAnd(t *testing.T) {
	t.Run("every branch must pass", func(t *testing.T) {
		rules := Rules{"n": Rules{OpAnd: []any{"natural", "min:10"}}}
		ok(t, rules, Data{"n": 12})
		fails(t, rules, Data{"n": 5}, []string{"n must be at least 10"})
	})
	t.Run("optional makes a nested object optional", func(t *testing.T) {
		rules := Rules{"billing": Rules{OpAnd: []any{
			"optional", Rules{"line1": "string|min:5", "city": "name"}}}}
		ok(t, rules, Data{})
		fails(t, rules, Data{"billing": Data{"line1": "x", "city": "London"}},
			[]string{"billing.line1 must have length of at least 5"})
	})
	t.Run("optional makes a tuple rule optional", func(t *testing.T) {
		rules := Rules{"products": Rules{OpAnd: []any{
			"optional", []Rules{{"title": "string|min:5", "price": "positive"}}}}}
		ok(t, rules, Data{})
		fails(t, rules, Data{"products": []any{Data{"title": "short", "price": -1}}},
			[]string{"products[0].price must be a valid positive number"})
	})
	t.Run("a custom rule combines with built-ins", func(t *testing.T) {
		even := func(v, _ any) *RuleError {
			n, _ := toNumber(v)
			if int64(n.Float)%2 == 0 {
				return nil
			}
			return &RuleError{Message: "must be even", Code: "NOT_EVEN"}
		}
		rules := Rules{"n": Rules{OpAnd: []any{"natural", CustomRule(even)}}}
		ok(t, rules, Data{"n": 4})
		fails(t, rules, Data{"n": 3}, []string{"must be even"})
	})
}

func TestSwitch(t *testing.T) {
	rules := Rules{"amount": Rules{OpSwitch: []any{
		map[string]any{"case": "number|max:1000", "then": "positive", "default": true},
		map[string]any{"case": "number|min:1001", "then": "positive|decimalmax:2"},
	}}}

	t.Run("a small valid amount takes the first case", func(t *testing.T) {
		ok(t, rules, Data{"amount": 500})
	})
	t.Run("a small invalid amount reports the first then", func(t *testing.T) {
		fails(t, rules, Data{"amount": -5}, []string{"amount must be a valid positive number"})
	})
	t.Run("a large valid amount takes the second case", func(t *testing.T) {
		ok(t, rules, Data{"amount": 5000})
	})
	t.Run("too many decimals reports the second then", func(t *testing.T) {
		fails(t, rules, Data{"amount": 5000.123}, []string{"amount must have at most 2 decimal places"})
	})
	t.Run("a non-number falls back to the default branch", func(t *testing.T) {
		fails(t, rules, Data{"amount": "abc"}, []string{"amount must be a valid number"})
	})
	t.Run("an absent value falls back to the default branch", func(t *testing.T) {
		fails(t, rules, Data{}, []string{"amount is required"})
	})
	t.Run("boundaries", func(t *testing.T) {
		ok(t, rules, Data{"amount": 1000})
		ok(t, rules, Data{"amount": 1001})
	})
	t.Run("the first matching case wins", func(t *testing.T) {
		r := Rules{"a": Rules{OpSwitch: []any{
			map[string]any{"case": "number", "then": "min:100"},
			map[string]any{"case": "number", "then": "max:1"},
		}}}
		fails(t, r, Data{"a": 5}, []string{"a must be at least 100"})
	})
	t.Run("no case and no default reports NO_CASE_MATCHED", func(t *testing.T) {
		r := Rules{"a": Rules{OpSwitch: []any{map[string]any{"case": "number", "then": "positive"}}}}
		fails(t, r, Data{"a": "x"}, []string{"a does not match any case"})
	})
}

// --- custom rules ---------------------------------------------------------

func TestCustomRules(t *testing.T) {
	t.Run("nil means pass", func(t *testing.T) {
		ok(t, Rules{"n": CustomRule(func(any, any) *RuleError { return nil })}, Data{"n": "anything"})
	})
	t.Run("cross-field validation via parent", func(t *testing.T) {
		rules := Rules{
			"password": "string|min:8",
			"confirmPassword": CustomRule(func(v, parent any) *RuleError {
				p, _ := asMap(parent)
				if v == p["password"] {
					return nil
				}
				return &RuleError{Message: "passwords must match", Code: "PASSWORD_MISMATCH"}
			}),
		}
		ok(t, rules, Data{"password": "longenough", "confirmPassword": "longenough"})
		fails(t, rules, Data{"password": "longenough", "confirmPassword": "different"},
			[]string{"passwords must match"})
	})
	t.Run("the function runs even when the value is absent", func(t *testing.T) {
		called := false
		rules := Rules{"n": CustomRule(func(v, _ any) *RuleError {
			called = true
			if v == nil {
				return &RuleError{Message: "n is needed", Code: "NEEDED"}
			}
			return nil
		})}
		fails(t, rules, Data{}, []string{"n is needed"})
		if !called {
			t.Fatal("custom rule was not called for an absent value")
		}
	})
	t.Run("the code passes through unchanged", func(t *testing.T) {
		rules := Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Message: "nope", Code: "MY_OWN_CODE"}
		})}
		if got := codesOf(t, rules, Data{"n": 1}); !reflect.DeepEqual(got, []ErrorCode{"MY_OWN_CODE"}) {
			t.Fatalf("expected MY_OWN_CODE, got %v", got)
		}
	})
	t.Run("a plain func literal is accepted", func(t *testing.T) {
		rules := Rules{"n": func(v, _ any) *RuleError {
			if v == nil {
				return &RuleError{Message: "missing", Code: "X"}
			}
			return nil
		}}
		ok(t, rules, Data{"n": 1})
	})
	t.Run("an empty message is a rule error", func(t *testing.T) {
		throws(t, Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Code: "X"}
		})}, Data{"n": 1})
	})
}

// --- groups ---------------------------------------------------------------

func TestAtleastAtmost(t *testing.T) {
	t.Run("$atleast passes when one is present", func(t *testing.T) {
		rules := Rules{"mail": "optional|email", "phone": "optional|phone", "$atleast": "mail|phone"}
		ok(t, rules, Data{"mail": "a@b.com"})
		fails(t, rules, Data{}, []string{"at least one of mail and phone is required"})
	})
	t.Run("group messages honour the quote style", func(t *testing.T) {
		rules := Rules{"mail": "optional|email", "phone": "optional|phone", "$atleast": "mail|phone"}
		fails(t, rules, Data{}, []string{`at least one of "mail" and "phone" is required`},
			Config{Quotes: QuoteDouble})
	})
	t.Run("$atleast counts presence, not truthiness", func(t *testing.T) {
		rules := Rules{"a": "optional|boolean", "b": "optional|boolean", "$atleast": "a|b"}
		ok(t, rules, Data{"a": false, "b": false})
	})
	t.Run("$atleast honours size:", func(t *testing.T) {
		rules := Rules{
			"a": "optional|string", "b": "optional|string", "c": "optional|string",
			"$atleast": "a|b|c|size:2",
		}
		ok(t, rules, Data{"a": "x", "b": "y"})
		fails(t, rules, Data{"a": "x"},
			[]string{"at least 2 of a, b and c are required"})
	})
	t.Run("$atmost caps how many may be given", func(t *testing.T) {
		rules := Rules{"mail": "optional|email", "phone": "optional|phone", "$atmost": "mail|phone"}
		ok(t, rules, Data{"mail": "a@b.com"})
		fails(t, rules, Data{"mail": "a@b.com", "phone": "9876543210"},
			[]string{"at most one of mail and phone can be given"})
	})
	t.Run("the list form applies several groups", func(t *testing.T) {
		rules := Rules{
			"a": "optional|string", "b": "optional|string",
			"x": "optional|string", "y": "optional|string",
			"$atleast": []string{"a|b", "x|y"},
		}
		ok(t, rules, Data{"a": "1", "x": "2"})
		if errs := errorsOf(t, rules, Data{"a": "1"}); len(errs) != 1 {
			t.Fatalf("expected one group failure, got %v", errs)
		}
	})
}

// --- options --------------------------------------------------------------

func TestQuotes(t *testing.T) {
	rules := Rules{"name": "name"}
	data := Data{"name": "..."}

	for _, tc := range []struct {
		style QuoteStyle
		want  string
	}{
		{QuoteNone, "name must be a valid name"},
		{QuoteSingle, "'name' must be a valid name"},
		{QuoteDouble, `"name" must be a valid name`},
		{QuoteTick, "`name` must be a valid name"},
	} {
		fails(t, rules, data, []string{tc.want}, Config{Quotes: tc.style})
	}
}

func TestStrict(t *testing.T) {
	t.Run("reports an unexpected field", func(t *testing.T) {
		fails(t, Rules{"a": "string"}, Data{"a": "x", "b": 1},
			[]string{"b is not required"}, Config{Strict: true})
	})
	t.Run("passes when every field has a rule", func(t *testing.T) {
		ok(t, Rules{"a": "string"}, Data{"a": "x"}, Config{Strict: true})
	})
	t.Run("applies to nested objects", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": "string"}}, Data{"a": Data{"b": "x", "c": 1}},
			[]string{"a.c is not required"}, Config{Strict: true})
	})
	t.Run("is off by default", func(t *testing.T) {
		ok(t, Rules{"a": "string"}, Data{"a": "x", "b": 1})
	})
}

func TestFieldAndErrorOverrides(t *testing.T) {
	t.Run("field: renames the field in the message", func(t *testing.T) {
		fails(t, Rules{"age": "natural|field:person age"}, Data{"age": 4.5},
			[]string{"person age must be a valid natural number"})
		fails(t, Rules{"age": "natural|field:person age"}, Data{},
			[]string{"person age is required"})
	})
	t.Run("error: replaces the message but keeps the code", func(t *testing.T) {
		rules := Rules{"age": "natural|error:given age is not valid"}
		fails(t, rules, Data{"age": 4.5}, []string{"given age is not valid"})
		if got := codesOf(t, rules, Data{"age": 4.5}); !reflect.DeepEqual(got, []ErrorCode{CodeNotNatural}) {
			t.Fatalf("expected NOT_NATURAL, got %v", got)
		}
	})
	t.Run("a message containing a pipe needs the list form", func(t *testing.T) {
		fails(t, Rules{"name": []string{"name", "error:bad | worse"}}, Data{"name": "123"},
			[]string{"bad | worse"})
	})
}

// --- result shape ---------------------------------------------------------

func TestResultShape(t *testing.T) {
	t.Run("Errors is nil on success", func(t *testing.T) {
		result, err := Validate(Rules{"a": "string"}, Data{"a": "x"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Errors != nil || result.Details != nil {
			t.Fatalf("expected nil slices, got %#v", result)
		}
		if !result.Valid() {
			t.Fatal("expected Valid() to be true")
		}
	})
	t.Run("Errors and Details are index-aligned", func(t *testing.T) {
		result, err := Validate(Rules{"a": "string", "b": "natural"}, Data{"a": 1, "b": -1})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Errors) != len(result.Details) {
			t.Fatalf("length mismatch: %d vs %d", len(result.Errors), len(result.Details))
		}
		for i := range result.Errors {
			if result.Errors[i] != result.Details[i].Message {
				t.Fatalf("index %d misaligned", i)
			}
		}
	})
	t.Run("duplicate messages collapse", func(t *testing.T) {
		rules := Rules{"a": "string|error:same", "b": "string|error:same"}
		result, err := Validate(rules, Data{"a": 1, "b": 2})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Errors) != 1 {
			t.Fatalf("expected one message, got %v", result.Errors)
		}
	})
	t.Run("Details carry the field name", func(t *testing.T) {
		result, _ := Validate(Rules{"a": "string"}, Data{"a": 1})
		if result.Details[0].Field != "a" {
			t.Fatalf("expected field 'a', got %q", result.Details[0].Field)
		}
	})
	t.Run("one field reports at most one error", func(t *testing.T) {
		fails(t, Rules{"f": "string|min:5|max:2"}, Data{"f": 1}, []string{"f must be string"})
	})
	t.Run("nil data reports DATA_REQUIRED", func(t *testing.T) {
		result, err := Validate(Rules{"a": "string"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Details) != 1 || result.Details[0].Code != CodeDataRequired {
			t.Fatalf("expected DATA_REQUIRED, got %#v", result.Details)
		}
	})
}

func TestErrorCodes(t *testing.T) {
	cases := []struct {
		rules Rules
		data  Data
		want  ErrorCode
	}{
		{Rules{"f": "string"}, Data{}, CodeRequired},
		{Rules{"f": "string"}, Data{"f": 1}, CodeNotString},
		{Rules{"f": "number"}, Data{"f": "x"}, CodeNotNumber},
		{Rules{"f": "string|number"}, Data{"f": "x"}, CodeNotNumericString},
		{Rules{"f": "boolean"}, Data{"f": 1}, CodeNotBoolean},
		{Rules{"f": "array"}, Data{"f": 1}, CodeNotArray},
		{Rules{"f": "object"}, Data{"f": 1}, CodeNotObject},
		{Rules{"f": "email"}, Data{"f": "x"}, CodeNotEmail},
		{Rules{"f": "natural"}, Data{"f": -1}, CodeNotNatural},
		{Rules{"f": "int"}, Data{"f": 1.5}, CodeNotInteger},
		{Rules{"f": "string|size:3"}, Data{"f": "ab"}, CodeLengthMismatch},
		{Rules{"f": "number|size:3"}, Data{"f": 1}, CodeDigitsMismatch},
		{Rules{"f": "string|min:5"}, Data{"f": "ab"}, CodeTooShort},
		{Rules{"f": "string|max:1"}, Data{"f": "ab"}, CodeTooLong},
		{Rules{"f": "number|min:5"}, Data{"f": 1}, CodeTooSmall},
		{Rules{"f": "number|max:1"}, Data{"f": 5}, CodeTooLarge},
		{Rules{"f": "enums:a,b"}, Data{"f": "c"}, CodeEnumMismatch},
		{Rules{"f": "equal:5"}, Data{"f": "6"}, CodeNotEqual},
		{Rules{"f": "number|decimalsize:2"}, Data{"f": 1.5}, CodeDecimalSizeMismatch},
		{Rules{"f": "number|decimalmax:1"}, Data{"f": 1.55}, CodeDecimalTooMany},
	}

	for _, tc := range cases {
		got := codesOf(t, tc.rules, tc.data)
		if len(got) != 1 || got[0] != tc.want {
			t.Errorf("rules %v data %v: expected [%s], got %v", tc.rules, tc.data, tc.want, got)
		}
	}
}

// --- malformed rules ------------------------------------------------------

func TestInvalidRules(t *testing.T) {
	t.Run("an unknown rule is reported", func(t *testing.T) {
		throws(t, Rules{"age": "mim:18"}, Data{"age": 1})
	})
	t.Run("a bare typo is reported", func(t *testing.T) {
		throws(t, Rules{"age": "naturl"}, Data{"age": 1})
	})
	t.Run("the field name appears in the message", func(t *testing.T) {
		_, err := Validate(Rules{"age": "naturl"}, Data{"age": 1})
		if err == nil || !strings.Contains(err.Error(), "age") {
			t.Fatalf("expected the field name in %v", err)
		}
	})
	t.Run("arrayof: with no inner rule is reported", func(t *testing.T) {
		throws(t, Rules{"f": "arrayof:"}, Data{"f": []any{}})
	})
	t.Run("symbol is reported as unsupported", func(t *testing.T) {
		throws(t, Rules{"f": "symbol"}, Data{"f": 1})
	})
	t.Run("a nested rule typo names the full path", func(t *testing.T) {
		_, err := Validate(Rules{"a": Rules{"b": "naturl"}}, Data{"a": Data{"b": 1}})
		if err == nil || !strings.Contains(err.Error(), "a.b") {
			t.Fatalf("expected the nested path in %v", err)
		}
	})
	t.Run("an operator with two arms is reported", func(t *testing.T) {
		throws(t, Rules{"f": Operator{Or: []any{"string"}, And: []any{"string"}}}, Data{"f": "x"})
	})
	t.Run("a switch branch without a then is reported", func(t *testing.T) {
		throws(t, Rules{"f": Operator{Switch: []SwitchBranch{{Case: "number"}}}}, Data{"f": 1})
	})
	t.Run("two default branches are reported", func(t *testing.T) {
		throws(t, Rules{"f": Operator{Switch: []SwitchBranch{
			{Case: "number", Then: "positive", Default: true},
			{Case: "string", Then: "email", Default: true},
		}}}, Data{"f": 1})
	})
	t.Run("a tuple rule with two elements is reported", func(t *testing.T) {
		throws(t, Rules{"f": []Rules{{"a": "string"}, {"b": "string"}}}, Data{"f": []any{}})
	})
}

// --- the documented example ----------------------------------------------

func TestReadmeExample(t *testing.T) {
	rules := Rules{
		"name":     "fullname",
		"email":    "email",
		"password": "string|min:8",
		"age":      "optional|natural|min:18",
		"role":     "enums:admin,user,guest",
		"website":  "optional|url",
	}
	data := Data{
		"name":     "John",
		"email":    "not-an-email",
		"password": "abc",
		"age":      15,
		"role":     "superuser",
		"website":  "example.com",
	}

	fails(t, rules, data, []string{
		"name must be a valid fullname",
		"email must be a valid email",
		"password must have length of at least 8",
		"age must be at least 18",
		"role is invalid",
		"website must be a valid url",
	})
}

func TestStructureExample(t *testing.T) {
	rules := Rules{
		"address": Rules{
			"city":    "name",
			"pin":     "string|natural|size:6",
			"country": Rules{"code": "alpha|upper|size:2"},
		},
		"tags":        "array|min:2|arrayof:string|arrayof:max:10",
		"users":       []Rules{{"name": "name", "age": "natural"}},
		"coords":      "array|size:2",
		"coords[0]":   "number|min:-90|max:90",
		"coords[1]":   "number|min:-180|max:180",
		"history[-1]": "date",
		"matrix":      "arrayof:arrayof:number",
	}
	data := Data{
		"address": Data{"city": "Rock Port", "pin": "ABC", "country": Data{"code": "in"}},
		"tags":    []any{"ok", "waaaaaaaaaytoolong"},
		"users":   []any{Data{"name": "Jo", "age": 20}, Data{}},
		"coords":  []any{200, -0.12},
		"history": []any{"nope"},
		"matrix":  []any{[]any{1}, []any{"x"}},
	}

	fails(t, rules, data, []string{
		"address.pin must be a valid numeric string",
		"address.country.code must not contains lower case letters",
		"tags[1] must have length of at most 10",
		"users[1].name is required",
		"users[1].age is required",
		"coords[0] must be at most 90",
		"history[-1] must be a valid date",
		"matrix[1][0] must be a valid number",
	})
}
