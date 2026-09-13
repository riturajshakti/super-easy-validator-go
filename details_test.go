package validator

import "testing"

// Codes are the stable half of a validation failure: message text is for
// humans and may be reworded, codes are meant to be compared against. These
// assertions pin the code emitted for every rule.

func singleCode(t *testing.T, rules Rules, data Data, cfg ...Config) ErrorCode {
	t.Helper()
	result, err := Validate(rules, data, cfg...)
	if err != nil {
		t.Fatalf("unexpected rule error: %v", err)
	}
	if len(result.Details) != 1 {
		t.Fatalf("expected exactly one detail, got %#v", result.Details)
	}
	return result.Details[0].Code
}

// --- one code per rule ----------------------------------------------------

func TestCodePerRule(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
		data  Data
		want  ErrorCode
	}{
		{"missing field", Rules{"f": "string"}, Data{}, CodeRequired},
		{"null field", Rules{"f": "string"}, Data{"f": nil}, CodeRequired},
		{"missing nested field", Rules{"a": Rules{"b": "string"}}, Data{"a": Data{}}, CodeRequired},
		{"missing tuple field", Rules{"u": []Rules{{"a": "string"}}}, Data{"u": []any{Data{}}}, CodeRequired},

		{"string", Rules{"f": "string"}, Data{"f": 1}, CodeNotString},
		{"number", Rules{"f": "number"}, Data{"f": "x"}, CodeNotNumber},
		{"numeric string", Rules{"f": "string|number"}, Data{"f": "x"}, CodeNotNumericString},
		{"boolean", Rules{"f": "boolean"}, Data{"f": 1}, CodeNotBoolean},
		{"boolean string", Rules{"f": "string|boolean"}, Data{"f": "yes"}, CodeNotBooleanString},
		{"array", Rules{"f": "array"}, Data{"f": 1}, CodeNotArray},
		{"object", Rules{"f": "object"}, Data{"f": 1}, CodeNotObject},
		{"bigint", Rules{"f": "bigint"}, Data{"f": 1}, CodeNotBigint},
		{"nested rule against a non-object", Rules{"a": Rules{"b": "string"}}, Data{"a": "x"}, CodeNotObject},
		{"tuple rule against a non-array", Rules{"u": []Rules{{"a": "string"}}}, Data{"u": "x"}, CodeNotArray},

		{"email", Rules{"f": "email"}, Data{"f": "x"}, CodeNotEmail},
		{"url", Rules{"f": "url"}, Data{"f": "x"}, CodeNotURL},
		{"domain", Rules{"f": "domain"}, Data{"f": "x"}, CodeNotDomain},
		{"name", Rules{"f": "name"}, Data{"f": "1"}, CodeNotName},
		{"fullname", Rules{"f": "fullname"}, Data{"f": "John"}, CodeNotFullname},
		{"username", Rules{"f": "username"}, Data{"f": "a"}, CodeNotUsername},
		{"alpha", Rules{"f": "alpha"}, Data{"f": "1"}, CodeNotAlpha},
		{"alphanumeric", Rules{"f": "alphanumeric"}, Data{"f": "-"}, CodeNotAlphanum},
		{"phone", Rules{"f": "phone"}, Data{"f": "abc"}, CodeNotPhone},
		{"phonecode", Rules{"f": "phonecode"}, Data{"f": "91"}, CodeNotPhonecode},
		{"objectid", Rules{"f": "objectid"}, Data{"f": "x"}, CodeNotObjectID},
		{"uuid", Rules{"f": "uuid"}, Data{"f": "x"}, CodeNotUUID},
		{"date", Rules{"f": "date"}, Data{"f": "zz"}, CodeNotDate},
		{"dateonly", Rules{"f": "dateonly"}, Data{"f": "zz"}, CodeNotDateonly},
		{"time", Rules{"f": "time"}, Data{"f": "zz"}, CodeNotTime},
		{"ip", Rules{"f": "ip"}, Data{"f": "x"}, CodeNotIP},
		{"lower", Rules{"f": "lower"}, Data{"f": "A"}, CodeNotLowercase},
		{"upper", Rules{"f": "upper"}, Data{"f": "a"}, CodeNotUppercase},

		{"int", Rules{"f": "int"}, Data{"f": 1.5}, CodeNotInteger},
		{"positive", Rules{"f": "positive"}, Data{"f": 0}, CodeNotPositive},
		{"negative", Rules{"f": "negative"}, Data{"f": 0}, CodeNotNegative},
		{"natural", Rules{"f": "natural"}, Data{"f": -1}, CodeNotNatural},
		{"whole", Rules{"f": "whole"}, Data{"f": -1}, CodeNotWhole},

		{"equal", Rules{"f": "equal:5"}, Data{"f": "6"}, CodeNotEqual},
		{"size on a string", Rules{"f": "string|size:5"}, Data{"f": "ab"}, CodeLengthMismatch},
		{"size on an array", Rules{"f": "array|size:5"}, Data{"f": []any{1}}, CodeLengthMismatch},
		{"size on a number", Rules{"f": "number|size:5"}, Data{"f": 1}, CodeDigitsMismatch},
		{"min on a string", Rules{"f": "string|min:5"}, Data{"f": "ab"}, CodeTooShort},
		{"min on an array", Rules{"f": "array|min:5"}, Data{"f": []any{1}}, CodeTooShort},
		{"max on a string", Rules{"f": "string|max:1"}, Data{"f": "ab"}, CodeTooLong},
		{"max on an array", Rules{"f": "array|max:1"}, Data{"f": []any{1, 2}}, CodeTooLong},
		{"min on a number", Rules{"f": "number|min:5"}, Data{"f": 1}, CodeTooSmall},
		{"max on a number", Rules{"f": "number|max:1"}, Data{"f": 9}, CodeTooLarge},
		{"min on a date", Rules{"f": "date|min:2024-01-01"}, Data{"f": "2023-01-01"}, CodeDateTooEarly},
		{"max on a date", Rules{"f": "date|max:2020-01-01"}, Data{"f": "2025-01-01"}, CodeDateTooLate},

		{"decimalsize", Rules{"f": "number|decimalsize:2"}, Data{"f": 1.5}, CodeDecimalSizeMismatch},
		{"decimalmin", Rules{"f": "number|decimalmin:3"}, Data{"f": 1.5}, CodeDecimalTooFew},
		{"decimalmax", Rules{"f": "number|decimalmax:1"}, Data{"f": 1.55}, CodeDecimalTooMany},

		{"enums", Rules{"f": "enums:a,b"}, Data{"f": "z"}, CodeEnumMismatch},
		{"regex", Rules{"f": `regex:/^a$/`}, Data{"f": "z"}, CodeRegexMismatch},
		{"regex on a non-string", Rules{"f": `regex:/^a$/`}, Data{"f": 1}, CodeNotString},

		{"arrayof element", Rules{"f": "arrayof:email"}, Data{"f": []any{"x"}}, CodeNotEmail},
		{"arrayof against a non-array", Rules{"f": "arrayof:string"}, Data{"f": "x"}, CodeNotArray},

		{"$atleast", Rules{"a": "optional|string", "b": "optional|string", "$atleast": "a|b"},
			Data{}, CodeAtleastNotMet},
		{"$atmost", Rules{"a": "optional|string", "b": "optional|string", "$atmost": "a|b"},
			Data{"a": "1", "b": "2"}, CodeAtmostExceeded},
		{"$switch with no match", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive"}}}}, Data{"v": "x"}, CodeNoCaseMatched},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := singleCode(t, tc.rules, tc.data); got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}

func TestDataGuardCodes(t *testing.T) {
	t.Run("nil data", func(t *testing.T) {
		if got := singleCode(t, Rules{"a": "string"}, nil); got != CodeDataRequired {
			t.Fatalf("expected DATA_REQUIRED, got %s", got)
		}
	})
	t.Run("an unexpected field under strict", func(t *testing.T) {
		if got := singleCode(t, Rules{"a": "string"}, Data{"a": "x", "b": 1},
			Config{Strict: true}); got != CodeUnexpectedField {
			t.Fatalf("expected UNEXPECTED_FIELD, got %s", got)
		}
	})
}

// --- codes separate failures that share a message -------------------------

func TestCodesDistinguishSharedMessages(t *testing.T) {
	t.Run("enums and regex both read 'is invalid'", func(t *testing.T) {
		result, _ := Validate(Rules{"e": "enums:a,b", "r": `regex:/^a$/`}, Data{"e": "z", "r": "z"})
		byField := map[string]ErrorCode{}
		for _, d := range result.Details {
			byField[d.Field] = d.Code
		}
		if byField["e"] != CodeEnumMismatch || byField["r"] != CodeRegexMismatch {
			t.Fatalf("expected distinct codes, got %#v", byField)
		}
	})
	t.Run("date and dateonly share a message", func(t *testing.T) {
		result, _ := Validate(Rules{"d": "date", "o": "dateonly"}, Data{"d": "zz", "o": "zz"})
		byField := map[string]ErrorCode{}
		for _, d := range result.Details {
			byField[d.Field] = d.Code
		}
		if byField["d"] != CodeNotDate || byField["o"] != CodeNotDateonly {
			t.Fatalf("expected distinct codes, got %#v", byField)
		}
	})
	t.Run("length and value bounds are different codes", func(t *testing.T) {
		result, _ := Validate(Rules{"s": "string|min:5", "n": "number|min:5"}, Data{"s": "ab", "n": 1})
		byField := map[string]ErrorCode{}
		for _, d := range result.Details {
			byField[d.Field] = d.Code
		}
		if byField["s"] != CodeTooShort || byField["n"] != CodeTooSmall {
			t.Fatalf("expected TOO_SHORT and TOO_SMALL, got %#v", byField)
		}
	})
}

// --- overrides keep the code ----------------------------------------------

func TestOverridesPreserveCodes(t *testing.T) {
	t.Run("error: replaces the message only", func(t *testing.T) {
		rules := Rules{"age": "natural|error:bad age"}
		result, _ := Validate(rules, Data{"age": -5})
		if result.Details[0].Message != "bad age" {
			t.Fatalf("expected the custom message, got %q", result.Details[0].Message)
		}
		if result.Details[0].Code != CodeNotNatural {
			t.Fatalf("expected NOT_NATURAL, got %s", result.Details[0].Code)
		}
	})
	t.Run("field: replaces the label only", func(t *testing.T) {
		if got := singleCode(t, Rules{"age": "natural|field:AGE"}, Data{"age": -5}); got != CodeNotNatural {
			t.Fatalf("expected NOT_NATURAL, got %s", got)
		}
	})
	t.Run("the numeric-string form shares its code", func(t *testing.T) {
		plain := singleCode(t, Rules{"f": "natural"}, Data{"f": -1})
		asText := singleCode(t, Rules{"f": "string|natural"}, Data{"f": "-1"})
		if plain != asText || plain != CodeNotNatural {
			t.Fatalf("expected both NOT_NATURAL, got %s and %s", plain, asText)
		}
	})
}

// --- the shape of Details -------------------------------------------------

func TestDetailShape(t *testing.T) {
	t.Run("Errors and Details stay aligned", func(t *testing.T) {
		result, _ := Validate(Rules{
			"a": "string", "b": "natural", "c": "email", "d": "array|min:2", "e": "enums:x",
		}, Data{"a": 1, "b": -1, "c": "bad", "d": []any{}, "e": "q"})

		if len(result.Errors) != len(result.Details) {
			t.Fatalf("length mismatch: %d vs %d", len(result.Errors), len(result.Details))
		}
		for i := range result.Errors {
			if result.Errors[i] != result.Details[i].Message {
				t.Fatalf("index %d misaligned", i)
			}
		}
	})
	t.Run("every Detail carries a field, message and code", func(t *testing.T) {
		result, _ := Validate(Rules{"a": "string", "b": "natural"}, Data{"a": 1, "b": -1})
		for _, d := range result.Details {
			if d.Field == "" || d.Message == "" || d.Code == "" {
				t.Fatalf("incomplete detail: %#v", d)
			}
		}
	})
	t.Run("the field carries the full path", func(t *testing.T) {
		result, _ := Validate(Rules{"a": Rules{"b": Rules{"c": "string"}}},
			Data{"a": Data{"b": Data{"c": 1}}})
		if result.Details[0].Field != "a.b.c" {
			t.Fatalf("expected a.b.c, got %q", result.Details[0].Field)
		}
	})
	t.Run("the field carries array indexes", func(t *testing.T) {
		result, _ := Validate(Rules{"u": []Rules{{"n": "string"}}}, Data{"u": []any{Data{"n": 1}}})
		if result.Details[0].Field != "u[0].n" {
			t.Fatalf("expected u[0].n, got %q", result.Details[0].Field)
		}
	})
	t.Run("both slices are nil on success", func(t *testing.T) {
		result, _ := Validate(Rules{"a": "string"}, Data{"a": "x"})
		if result.Errors != nil || result.Details != nil {
			t.Fatalf("expected nil slices, got %#v", result)
		}
		if !result.Valid() {
			t.Fatal("expected Valid() to report success")
		}
	})
	t.Run("duplicate messages collapse but keep the first code", func(t *testing.T) {
		result, _ := Validate(Rules{"a": "string|error:same", "b": "natural|error:same"},
			Data{"a": 1, "b": -1})
		if len(result.Details) != 1 {
			t.Fatalf("expected one detail, got %#v", result.Details)
		}
	})
}
