package validator

import "testing"

// Malformed rules are programmer errors, not validation failures. Every case
// here must be reported as an *InvalidRuleError rather than silently ignored
// or treated as a failed validation.

// --- an unknown rule, in every position it can hide -----------------------

func TestUnknownRuleEveryPosition(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
		data  Data
	}{
		{"a plain field", Rules{"f": "naturl"}, Data{"f": 1}},
		{"the array rule form", Rules{"f": []string{"string", "naturl"}}, Data{"f": 1}},
		{"a nested object rule", Rules{"a": Rules{"b": "naturl"}}, Data{"a": Data{"b": 1}}},
		{"two levels deep", Rules{"a": Rules{"b": Rules{"c": "naturl"}}}, Data{"a": Data{"b": Data{"c": 1}}}},
		{"a tuple element rule", Rules{"u": []Rules{{"n": "naturl"}}}, Data{"u": []any{Data{"n": 1}}}},
		{"a nested tuple rule", Rules{"g": []any{[]Rules{{"n": "naturl"}}}}, Data{"g": []any{[]any{Data{"n": 1}}}}},
		{"a dotted key", Rules{"a.b": "naturl"}, Data{"a": Data{"b": 1}}},
		{"an indexed key", Rules{"c[0]": "naturl"}, Data{"c": []any{1}}},
		{"a negative index key", Rules{"c[-1]": "naturl"}, Data{"c": []any{1}}},
		{"a slice key", Rules{"c[0:2]": "naturl"}, Data{"c": []any{1, 2}}},
		{"a $or branch", Rules{"v": Rules{"$or": []any{"naturl", "string"}}}, Data{"v": 1}},
		{"a $and branch", Rules{"v": Rules{"$and": []any{"naturl"}}}, Data{"v": 1}},
		{"a $switch case", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "naturl", "then": "positive"}}}}, Data{"v": 1}},
		{"a $switch then", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "naturl"}}}}, Data{"v": 1}},
		{"an operator nested in an operator", Rules{"v": Rules{"$or": []any{
			Rules{"$and": []any{"naturl"}}}}}, Data{"v": 1}},
		{"an object branch of $or", Rules{"v": Rules{"$or": []any{Rules{"n": "naturl"}}}}, Data{"v": Data{"n": 1}}},
		{"a tuple branch of $and", Rules{"v": Rules{"$and": []any{"optional", []Rules{{"n": "naturl"}}}}},
			Data{"v": []any{Data{"n": 1}}}},
		{"an arrayof suffix", Rules{"f": "arrayof:naturl"}, Data{"f": []any{1}}},
		{"a nested arrayof suffix", Rules{"f": "arrayof:arrayof:naturl"}, Data{"f": []any{[]any{1}}}},
		{"an empty arrayof suffix", Rules{"f": "arrayof:"}, Data{"f": []any{}}},
		{"an unknown argument rule", Rules{"f": "mim:5"}, Data{"f": 1}},
		{"the dropped symbol rule", Rules{"f": "symbol"}, Data{"f": 1}},
		{"the dropped mongoid rule", Rules{"f": "mongoid"}, Data{"f": "x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { throws(t, tc.rules, tc.data) })
	}
}

// The reported error must name the offending field, so a typo in a deep rule
// tree is findable.
func TestUnknownRuleNamesTheField(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
		data  Data
		field string
	}{
		{"a plain field", Rules{"age": "naturl"}, Data{"age": 1}, "age"},
		{"a nested field", Rules{"a": Rules{"b": "naturl"}}, Data{"a": Data{"b": 1}}, "a.b"},
		{"three levels deep", Rules{"a": Rules{"b": Rules{"c": "naturl"}}},
			Data{"a": Data{"b": Data{"c": 1}}}, "a.b.c"},
		{"a tuple element", Rules{"u": []Rules{{"n": "naturl"}}}, Data{"u": []any{Data{"n": 1}}}, "u.n"},
		{"an operator branch", Rules{"v": Rules{"$or": []any{"naturl"}}}, Data{"v": 1}, "v"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Validate(tc.rules, tc.data)
			if err == nil {
				t.Fatal("expected an InvalidRuleError")
			}
			ruleErr, isRuleErr := err.(*InvalidRuleError)
			if !isRuleErr {
				t.Fatalf("expected *InvalidRuleError, got %T", err)
			}
			if ruleErr.Field != tc.field {
				t.Fatalf("expected field %q, got %q", tc.field, ruleErr.Field)
			}
		})
	}
}

// --- malformed $or and $and shapes ----------------------------------------

func TestMalformedOrAnd(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
	}{
		{"$or branches are an object", Rules{"v": Rules{"$or": Rules{"a": "string"}}}},
		{"$or branches are a string", Rules{"v": Rules{"$or": "string"}}},
		{"$or branches are a number", Rules{"v": Rules{"$or": 5}}},
		{"$or branches are empty", Rules{"v": Rules{"$or": []any{}}}},
		{"$and branches are empty", Rules{"v": Rules{"$and": []any{}}}},
		{"$and branches are a string", Rules{"v": Rules{"$and": "string"}}},
		{"$or and $and together", Rules{"v": Rules{"$or": []any{"string"}, "$and": []any{"string"}}}},
		{"$or and $switch together", Rules{"v": Rules{"$or": []any{"string"}, "$switch": []any{}}}},
		{"$and and $switch together", Rules{"v": Rules{"$and": []any{"string"}, "$switch": []any{}}}},
		{"$or beside a sibling key", Rules{"v": Rules{"$or": []any{"string"}, "x": "string"}}},
		{"$and beside a sibling key", Rules{"v": Rules{"$and": []any{"string"}, "x": "string"}}},
		{"$switch beside a sibling key", Rules{"v": Rules{
			"$switch": []any{map[string]any{"case": "number", "then": "positive"}}, "x": "string"}}},
		{"a $or branch that is only optional", Rules{"v": Rules{"$or": []any{"optional"}}}},
		{"a $or branch that is only nullable", Rules{"v": Rules{"$or": []any{"nullable"}}}},
		{"a $or branch that is only optional|nullable", Rules{"v": Rules{"$or": []any{"optional|nullable"}}}},
		{"a $or branch that is a modifier-only list", Rules{"v": Rules{"$or": []any{[]string{"optional", "nullable"}}}}},
		{"a multi-arm Operator struct", Rules{"v": Operator{Or: []any{"string"}, And: []any{"string"}}}},
		{"a three-arm Operator struct", Rules{"v": Operator{
			Or: []any{"string"}, And: []any{"string"},
			Switch: []SwitchBranch{{Case: "number", Then: "positive"}}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { throws(t, tc.rules, Data{"v": 1}) })
	}
}

// An $and branch may be only optional: unlike $or, it does not swallow the
// whole check.
func TestAndModifierOnlyBranchIsLegal(t *testing.T) {
	ok(t, Rules{"v": Rules{"$and": []any{"optional"}}}, Data{})
	ok(t, Rules{"v": Rules{"$and": []any{"nullable"}}}, Data{"v": nil})
}

// --- malformed $switch shapes ---------------------------------------------

func TestMalformedSwitch(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
	}{
		{"branches are a string", Rules{"v": Rules{"$switch": "x"}}},
		{"branches are empty", Rules{"v": Rules{"$switch": []any{}}}},
		{"a branch is a string", Rules{"v": Rules{"$switch": []any{"number"}}}},
		{"a branch is an array", Rules{"v": Rules{"$switch": []any{[]any{"number"}}}}},
		{"a branch is nil", Rules{"v": Rules{"$switch": []any{nil}}}},
		{"a branch has no case", Rules{"v": Rules{"$switch": []any{
			map[string]any{"then": "positive"}}}}},
		{"a branch has no then", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number"}}}}},
		{"a branch has an unknown key", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "oops": 1}}}}},
		{"default is a string", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "default": "yes"}}}}},
		{"default is a number", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "default": 1}}}}},
		{"two default branches", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "default": true},
			map[string]any{"case": "string", "then": "email", "default": true}}}}},
		{"a Switch struct branch with no then", Rules{"v": Operator{
			Switch: []SwitchBranch{{Case: "number"}}}}},
		{"two default Switch struct branches", Rules{"v": Operator{Switch: []SwitchBranch{
			{Case: "number", Then: "positive", Default: true},
			{Case: "string", Then: "email", Default: true}}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { throws(t, tc.rules, Data{"v": 1}) })
	}
}

// default: false is a legal, ordinary branch.
func TestSwitchDefaultFalseIsLegal(t *testing.T) {
	ok(t, Rules{"v": Rules{"$switch": []any{
		map[string]any{"case": "number", "then": "positive", "default": false}}}}, Data{"v": 5})
}

// --- malformed rule values ------------------------------------------------

func TestMalformedRuleValues(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
	}{
		{"a nil rule value", Rules{"f": nil}},
		{"an unsupported rule type", Rules{"f": 42}},
		{"a bool rule value", Rules{"f": true}},
		{"a tuple rule with two elements", Rules{"f": []Rules{{"a": "string"}, {"b": "string"}}}},
		{"a nested tuple rule with two elements", Rules{"f": []any{[]Rules{{"a": "string"}}, []Rules{{"b": "string"}}}}},
		{"a regex using lookahead", Rules{"f": `regex:/^(?!www)x$/`}},
		{"a regex using negative lookbehind", Rules{"f": `regex:/(?<!a)b/`}},
		{"a regex using a backreference", Rules{"f": `regex:/^(a)\1$/`}},
		{"a regex with an unsupported flag", Rules{"f": `regex:/^a$/q`}},
		{"an unterminated regex literal", Rules{"f": `regex:/^a$`}},
		{"an invalid regex", Rules{"f": `regex:/^[a$/`}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { throws(t, tc.rules, Data{"f": "x"}) })
	}
}

// A rule error is reported before any data is examined, so it surfaces even
// when the data would never reach the offending rule.
func TestRuleErrorsSurfaceBeforeData(t *testing.T) {
	t.Run("when the field is absent", func(t *testing.T) {
		throws(t, Rules{"f": "naturl"}, Data{})
	})
	t.Run("when a $switch case would never be reached", func(t *testing.T) {
		throws(t, Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive"},
			map[string]any{"case": "string", "then": "naturl"}}}}, Data{"v": 5})
	})
	t.Run("when a $or branch would not be needed", func(t *testing.T) {
		throws(t, Rules{"v": Rules{"$or": []any{"number", "naturl"}}}, Data{"v": 5})
	})
	t.Run("when a tuple rule has no elements to check", func(t *testing.T) {
		throws(t, Rules{"u": []Rules{{"n": "naturl"}}}, Data{"u": []any{}})
	})
}
