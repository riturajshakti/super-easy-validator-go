package validator

import "testing"

// The assertions in this file were taken from the npm package's observed
// output, by running the same rules and data through super-easy-validator
// v0.9.0 and recording what it produced.

// --- operators as map keys ------------------------------------------------

func TestOperatorKeys(t *testing.T) {
	t.Run("$or key accepts either branch", func(t *testing.T) {
		rules := Rules{"id": Rules{"$or": []any{"objectid", "uuid"}}}
		ok(t, rules, Data{"id": "507f1f77bcf86cd799439011"})
		ok(t, rules, Data{"id": "123e4567-e89b-12d3-a456-426655440000"})
		fails(t, rules, Data{"id": "nope"}, []string{"id must be a valid object id"})
	})
	t.Run("$or key with an object branch", func(t *testing.T) {
		rules := Rules{"address": Rules{"$or": []any{
			"string|max:60", Rules{"city": "name", "pin": "string|natural|size:6"}}}}
		fails(t, rules, Data{"address": Data{"city": "123", "pin": "123456"}},
			[]string{"address.city must be a valid name"})
	})
	t.Run("$and key makes a nested object optional", func(t *testing.T) {
		rules := Rules{"billing": Rules{"$and": []any{"optional", Rules{"line1": "string|min:5"}}}}
		ok(t, rules, Data{})
		fails(t, rules, Data{"billing": Data{"line1": "x"}},
			[]string{"billing.line1 must have length of at least 5"})
	})
	t.Run("$and key reports every branch failure", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$and": []any{"string", "min:10"}}}, Data{"v": "AB"},
			[]string{"v must have length of at least 10"})
	})
	t.Run("$switch key", func(t *testing.T) {
		rules := Rules{"amount": Rules{"$switch": []any{
			map[string]any{"case": "number|max:1000", "then": "positive", "default": true},
			map[string]any{"case": "number|min:1001", "then": "positive|decimalmax:2"},
		}}}
		ok(t, rules, Data{"amount": 500})
		fails(t, rules, Data{"amount": -5}, []string{"amount must be a valid positive number"})
		fails(t, rules, Data{"amount": "abc"}, []string{"amount must be a valid number"})
	})
	t.Run("an operator nested inside an operator", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$and": []any{"optional", Rules{"$or": []any{"email", "phone"}}}}},
			Data{"v": "bad"}, []string{"v must be a valid email"})
	})
	t.Run("operators nest three deep", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$or": []any{Rules{"$and": []any{Rules{"$or": []any{"email", "phone"}}}}}}},
			Data{"v": "bad"}, []string{"v must be a valid email"})
	})
	t.Run("an operator inside a nested object rule", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": Rules{"$or": []any{"email", "phone"}}}},
			Data{"a": Data{"b": "bad"}}, []string{"a.b must be a valid email"})
	})
	t.Run("an operator inside a tuple rule", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"c": Rules{"$or": []any{"email", "phone"}}}}},
			Data{"u": []any{Data{"c": "bad"}}}, []string{"u[0].c must be a valid email"})
	})
	t.Run("an operator branch always type-matches", func(t *testing.T) {
		rules := Rules{"v": Rules{"$or": []any{
			Rules{"$and": []any{"natural", "min:10"}}, Rules{"a": "string"}}}}
		fails(t, rules, Data{"v": 5}, []string{"v must be at least 10"})
	})
	t.Run("a plain map[string]any operator node works", func(t *testing.T) {
		fails(t, Rules{"id": map[string]any{"$or": []any{"objectid", "uuid"}}},
			Data{"id": "nope"}, []string{"id must be a valid object id"})
	})
	t.Run("the Operator struct remains equivalent", func(t *testing.T) {
		fails(t, Rules{"id": Operator{Or: []any{"objectid", "uuid"}}},
			Data{"id": "nope"}, []string{"id must be a valid object id"})
		fails(t, Rules{"amount": Operator{Switch: []SwitchBranch{
			{Case: "number|max:1000", Then: "positive", Default: true}}}},
			Data{"amount": -5}, []string{"amount must be a valid positive number"})
	})
}

// --- $and object-branch merging -------------------------------------------

func TestAndMerging(t *testing.T) {
	two := Rules{"v": Rules{"$and": []any{Rules{"a": "string"}, Rules{"b": "string"}}}}

	t.Run("merging does not change non-strict output", func(t *testing.T) {
		fails(t, two, Data{"v": Data{"a": 1, "b": 2}},
			[]string{"v.a must be string", "v.b must be string"})
		fails(t, two, Data{"v": Data{"a": "x", "b": 2}}, []string{"v.b must be string"})
		fails(t, two, Data{"v": Data{}}, []string{"v.a is required", "v.b is required"})
	})
	t.Run("keys from every branch count as declared under strict", func(t *testing.T) {
		ok(t, two, Data{"v": Data{"a": "x", "b": "y"}}, Config{Strict: true})
		fails(t, two, Data{"v": Data{"a": "x", "b": "y", "c": 1}},
			[]string{"v.c is not required"}, Config{Strict: true})
	})
	t.Run("a single object branch still strict-checks", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$and": []any{Rules{"a": "string"}}}},
			Data{"v": Data{"a": "x", "z": 1}}, []string{"v.z is not required"}, Config{Strict: true})
	})
	t.Run("optional combines with several object branches", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$and": []any{"optional", Rules{"a": "string"}, Rules{"b": "string"}}}},
			Data{"v": Data{"a": 1, "b": 2}},
			[]string{"v.a must be string", "v.b must be string"})
	})
	t.Run("optional combines with a tuple branch", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$and": []any{"optional", []Rules{{"a": "string"}}}}},
			Data{"v": []any{Data{"a": 1}}}, []string{"v[0].a must be string"})
	})
}

// --- $or branch selection -------------------------------------------------

func TestOrSelection(t *testing.T) {
	t.Run("$or reports the nearest branch for a scalar", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$or": []any{"email", "phone"}}}, Data{"v": 12345},
			[]string{"v must be string"})
	})
	t.Run("an array branch is chosen for array data", func(t *testing.T) {
		fails(t, Rules{"v": Rules{"$or": []any{"string", "arrayof:number"}}},
			Data{"v": []any{1, "x"}}, []string{"v[1] must be a valid number"})
	})
	t.Run("$or of two operator branches", func(t *testing.T) {
		rules := Rules{"v": Rules{"$or": []any{
			Rules{"$switch": []any{map[string]any{"case": "number", "then": "positive", "default": true}}},
			"email"}}}
		fails(t, rules, Data{"v": -5}, []string{"v must be a valid positive number"})
	})
	t.Run("$or under strict ignores unexpected-only failures", func(t *testing.T) {
		ok(t, Rules{"v": Rules{"$or": []any{Rules{"a": "string"}, Rules{"b": "string"}}}},
			Data{"v": Data{"a": "x"}}, Config{Strict: true})
	})
}

// --- $switch --------------------------------------------------------------

func TestSwitchBranches(t *testing.T) {
	t.Run("a function may be the case", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{map[string]any{
			"case": CustomRule(func(x, _ any) *RuleError {
				if isNumericKind(x) {
					return nil
				}
				return &RuleError{Message: "nan", Code: "N"}
			}),
			"then": "positive"}}}}
		fails(t, rules, Data{"v": -5}, []string{"v must be a valid positive number"})
	})
	t.Run("a function may be the then", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{map[string]any{
			"case": "number",
			"then": CustomRule(func(x, _ any) *RuleError {
				n, _ := toNumber(x)
				if n.Float > 0 {
					return nil
				}
				return &RuleError{Message: "pos!", Code: "P"}
			})}}}}
		fails(t, rules, Data{"v": -5}, []string{"pos!"})
	})
	t.Run("the default need not be the first branch", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "string", "then": "email"},
			map[string]any{"case": "number", "then": "positive", "default": true}}}}
		fails(t, rules, Data{"v": true}, []string{"v must be a valid number"})
	})
	t.Run("an object rule may be the then", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "object", "then": Rules{"a": "string"}, "default": true}}}}
		fails(t, rules, Data{"v": Data{"a": 1}}, []string{"v.a must be string"})
	})
	t.Run("a tuple rule may be the then", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "array", "then": []Rules{{"a": "string"}}, "default": true}}}}
		fails(t, rules, Data{"v": []any{Data{"a": 1}}}, []string{"v[0].a must be string"})
	})
	t.Run("arrayof may be the then", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "array", "then": "arrayof:number", "default": true}}}}
		fails(t, rules, Data{"v": []any{1, "x"}}, []string{"v[1] must be a valid number"})
	})
}

// --- nested tuple rules ---------------------------------------------------

func TestNestedTuples(t *testing.T) {
	grid := Rules{"grid": []any{[]Rules{{"label": "string"}}}}

	t.Run("an array of arrays of objects validates at depth", func(t *testing.T) {
		ok(t, grid, Data{"grid": []any{[]any{Data{"label": "x"}}}})
		fails(t, grid, Data{"grid": []any{[]any{Data{"label": 1}}}},
			[]string{"grid[0][0].label must be string"})
	})
	t.Run("the row index appears in the path", func(t *testing.T) {
		fails(t, grid, Data{"grid": []any{[]any{Data{"label": "a"}}, []any{Data{"label": 2}}}},
			[]string{"grid[1][0].label must be string"})
	})
	t.Run("a non-array outer value", func(t *testing.T) {
		fails(t, grid, Data{"grid": "x"}, []string{"grid must be of type array"})
	})
	t.Run("a non-array inner value", func(t *testing.T) {
		fails(t, grid, Data{"grid": []any{"x"}}, []string{"grid[0] must be of type array"})
	})
	t.Run("a non-object leaf", func(t *testing.T) {
		fails(t, grid, Data{"grid": []any{[]any{"x"}}}, []string{"grid[0][0] must be of type object/array"})
	})
	t.Run("a null tuple element reports required", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"a": "string"}}}, Data{"u": []any{nil}},
			[]string{"u[0] is required"})
	})
	t.Run("a scalar tuple element reports a type error", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"a": "string"}}}, Data{"u": []any{"x"}},
			[]string{"u[0] must be of type object/array"})
	})
	t.Run("an empty array passes a tuple rule", func(t *testing.T) {
		ok(t, Rules{"u": []Rules{{"a": "string"}}}, Data{"u": []any{}})
	})
	t.Run("arrayof inside a tuple", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"tags": "arrayof:string"}}},
			Data{"u": []any{Data{"tags": []any{"a", 1}}}}, []string{"u[0].tags[1] must be string"})
	})
	t.Run("a nested object inside a tuple", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"addr": Rules{"city": "name"}}}},
			Data{"u": []any{Data{"addr": Data{"city": "123"}}}},
			[]string{"u[0].addr.city must be a valid name"})
	})
	t.Run("object, array, object, array", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": []Rules{{"c": "arrayof:number"}}}},
			Data{"a": Data{"b": []any{Data{"c": []any{1, "x"}}}}},
			[]string{"a.b[0].c[1] must be a valid number"})
	})
}

// --- malformed operator nodes ---------------------------------------------

func TestMalformedOperators(t *testing.T) {
	cases := []struct {
		name  string
		rules Rules
	}{
		{"an operator beside another key", Rules{"v": Rules{"$or": []any{"string"}, "city": "name"}}},
		{"two operators in one object", Rules{"v": Rules{"$or": []any{"string"}, "$and": []any{"string"}}}},
		{"branches are not an array", Rules{"v": Rules{"$or": "string"}}},
		{"no branches", Rules{"v": Rules{"$or": []any{}}}},
		{"a branch that is only optional", Rules{"v": Rules{"$or": []any{"optional", "string"}}}},
		{"a switch branch with an unknown key", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "oops": 1}}}}},
		{"a switch branch that is not an object", Rules{"v": Rules{"$switch": []any{"number"}}}},
		{"a switch default that is not a boolean", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "default": "yes"}}}}},
		{"a switch branch with no case", Rules{"v": Rules{"$switch": []any{
			map[string]any{"then": "positive"}}}}},
		{"a switch branch with no then", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number"}}}}},
		{"two default branches", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "number", "then": "positive", "default": true},
			map[string]any{"case": "string", "then": "email", "default": true}}}}},
		{"an unknown rule inside a branch", Rules{"v": Rules{"$or": []any{"naturl"}}}},
		{"an unknown rule inside a case", Rules{"v": Rules{"$switch": []any{
			map[string]any{"case": "naturl", "then": "positive"}}}}},
		{"a multi-arm Operator struct", Rules{"v": Operator{Or: []any{"string"}, And: []any{"string"}}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { throws(t, tc.rules, Data{"v": "x"}) })
	}
}

// --- indexing -------------------------------------------------------------

func TestIndexingCorners(t *testing.T) {
	t.Run("an open-ended slice", func(t *testing.T) {
		fails(t, Rules{"c[1:]": "number"}, Data{"c": []any{1, "x", "y"}},
			[]string{"c[1] must be a valid number", "c[2] must be a valid number"})
	})
	t.Run("an open-start slice", func(t *testing.T) {
		fails(t, Rules{"c[:2]": "number"}, Data{"c": []any{"x", "y", 3}},
			[]string{"c[0] must be a valid number", "c[1] must be a valid number"})
	})
	t.Run("a negative slice", func(t *testing.T) {
		fails(t, Rules{"c[-2:]": "number"}, Data{"c": []any{1, "x", "y"}},
			[]string{"c[1] must be a valid number", "c[2] must be a valid number"})
	})
	t.Run("an out-of-range slice selects nothing", func(t *testing.T) {
		ok(t, Rules{"c[5:9]": "number"}, Data{"c": []any{1}})
	})
	t.Run("chained indexes", func(t *testing.T) {
		fails(t, Rules{"c[0][1]": "number"}, Data{"c": []any{[]any{1, "x"}}},
			[]string{"c[0][1] must be a valid number"})
	})
	t.Run("a slice followed by a property", func(t *testing.T) {
		fails(t, Rules{"u[0:2].name": "string"}, Data{"u": []any{Data{"name": 1}, Data{"name": 2}}},
			[]string{"u[0].name must be string", "u[1].name must be string"})
	})
	t.Run("an index followed by a property", func(t *testing.T) {
		fails(t, Rules{"u[0].name": "string"}, Data{"u": []any{Data{"name": 1}}},
			[]string{"u[0].name must be string"})
	})
	t.Run("a custom rule on an indexed key", func(t *testing.T) {
		rules := Rules{"u[0]": CustomRule(func(x, _ any) *RuleError {
			n, _ := toNumber(x)
			if n.Float == 1 {
				return nil
			}
			return &RuleError{Message: "bad idx", Code: "I"}
		})}
		fails(t, rules, Data{"u": []any{2}}, []string{"bad idx"})
	})
	t.Run("an indexed rule declares its base key under strict", func(t *testing.T) {
		ok(t, Rules{"c[0]": "number"}, Data{"c": []any{1}}, Config{Strict: true})
	})
}

// --- constraints ----------------------------------------------------------

func TestConstraintCorners(t *testing.T) {
	t.Run("size and min ignore a boolean", func(t *testing.T) {
		ok(t, Rules{"f": "size:3"}, Data{"f": true})
		ok(t, Rules{"f": "min:3"}, Data{"f": true})
	})
	t.Run("equal compares booleans", func(t *testing.T) {
		ok(t, Rules{"f": "equal:true"}, Data{"f": true})
		fails(t, Rules{"f": "equal:true"}, Data{"f": false}, []string{"f must be equal to true"})
	})
	t.Run("enums compares booleans", func(t *testing.T) {
		fails(t, Rules{"f": "boolean|enums:true"}, Data{"f": false}, []string{"f is invalid"})
	})
	t.Run("regex on a non-string reports a type error", func(t *testing.T) {
		fails(t, Rules{"f": `regex:/^a$/`}, Data{"f": 5}, []string{"f must be of type string"})
	})
	t.Run("zero-valued constraints", func(t *testing.T) {
		ok(t, Rules{"f": "number|decimalsize:0"}, Data{"f": 5})
		ok(t, Rules{"f": "number|decimalmin:0"}, Data{"f": 5})
		ok(t, Rules{"f": "string|size:0"}, Data{"f": ""})
		ok(t, Rules{"f": "array|min:0"}, Data{"f": []any{}})
	})
	t.Run("negative bounds", func(t *testing.T) {
		fails(t, Rules{"f": "number|max:-5"}, Data{"f": -3}, []string{"f must be at most -5"})
		fails(t, Rules{"f": "number|min:-10"}, Data{"f": -20}, []string{"f must be at least -10"})
	})
	t.Run("equal ignores a value that is not string, number or boolean", func(t *testing.T) {
		ok(t, Rules{"f": "equal:5"}, Data{"f": []any{5}})
	})
	t.Run("an empty enum member matches an empty string", func(t *testing.T) {
		ok(t, Rules{"f": "enums:a,,b"}, Data{"f": ""})
	})
	t.Run("a numeric string is not a number enum", func(t *testing.T) {
		fails(t, Rules{"f": "number|enums:1,2"}, Data{"f": "1"}, []string{"f must be a valid number"})
	})
	t.Run("size counts runes, not bytes", func(t *testing.T) {
		fails(t, Rules{"f": "string|size:3"}, Data{"f": "héllo"}, []string{"f must have length 3"})
	})
	t.Run("negative zero is not natural", func(t *testing.T) {
		fails(t, Rules{"f": "natural"}, Data{"f": -0.0}, []string{"f must be a valid natural number"})
	})
}

// --- arrayof --------------------------------------------------------------

func TestArrayOfCorners(t *testing.T) {
	t.Run("an empty array passes", func(t *testing.T) {
		ok(t, Rules{"f": "arrayof:string"}, Data{"f": []any{}})
	})
	t.Run("null is not an array", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:string"}, Data{"f": nil}, []string{"f must be an array"})
	})
	t.Run("arrayof:array", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:array"}, Data{"f": []any{[]any{1}, "x"}},
			[]string{"f[1] must be an array"})
	})
	t.Run("arrayof with argument rules", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:enums:a,b"}, Data{"f": []any{"a", "c"}}, []string{"f[1] is invalid"})
		fails(t, Rules{"f": `arrayof:regex:/^a$/`}, Data{"f": []any{"a", "b"}}, []string{"f[1] is invalid"})
		fails(t, Rules{"f": "arrayof:equal:5"}, Data{"f": []any{5, 6}}, []string{"f[1] must be equal to 5"})
	})
	t.Run("arrayof with a string format", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:objectid"}, Data{"f": []any{"507f1f77bcf86cd799439011", "x"}},
			[]string{"f[1] must be a valid object id"})
	})
	t.Run("arrayof:nullable allows null elements", func(t *testing.T) {
		ok(t, Rules{"f": "arrayof:nullable|arrayof:string"}, Data{"f": []any{nil, "a"}})
	})
	t.Run("three levels deep", func(t *testing.T) {
		fails(t, Rules{"cube": "arrayof:arrayof:arrayof:number"},
			Data{"cube": []any{[]any{[]any{"x"}}}}, []string{"cube[0][0][0] must be a valid number"})
	})
	t.Run("every element failure is reported", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:number"}, Data{"f": []any{"a", "b"}},
			[]string{"f[0] must be a valid number", "f[1] must be a valid number"})
	})
}

// --- strict ---------------------------------------------------------------

func TestStrictCorners(t *testing.T) {
	t.Run("strict reaches into a tuple element", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"a": "string"}}}, Data{"u": []any{Data{"a": "x", "z": 1}}},
			[]string{"u[0].z is not required"}, Config{Strict: true})
	})
	t.Run("a dotted rule key does not declare its base", func(t *testing.T) {
		fails(t, Rules{"a.b": "string"}, Data{"a": Data{"b": "x"}},
			[]string{"a is not required"}, Config{Strict: true})
	})
	t.Run("group keys are not treated as fields", func(t *testing.T) {
		ok(t, Rules{"a": "optional|string", "b": "optional|string", "$atleast": "a|b"},
			Data{"a": "x"}, Config{Strict: true})
	})
	t.Run("strict applies at every depth", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": Rules{"c": "string"}}},
			Data{"a": Data{"b": Data{"c": "x", "d": 1}}},
			[]string{"a.b.d is not required"}, Config{Strict: true})
	})
	t.Run("empty rules reject every field under strict", func(t *testing.T) {
		fails(t, Rules{}, Data{"a": 1}, []string{"a is not required"}, Config{Strict: true})
		ok(t, Rules{}, Data{"a": 1})
	})
}

// --- messages -------------------------------------------------------------

func TestMessageCorners(t *testing.T) {
	t.Run("quotes apply to a nested path", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": "string"}}, Data{"a": Data{"b": 1}},
			[]string{"`a.b` must be string"}, Config{Quotes: QuoteTick})
	})
	t.Run("quotes apply to an array index", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:string"}, Data{"f": []any{1}},
			[]string{"'f[0]' must be string"}, Config{Quotes: QuoteSingle})
	})
	t.Run("quotes apply to a group message", func(t *testing.T) {
		fails(t, Rules{"a": "optional|string", "b": "optional|string", "$atleast": "a|b"}, Data{},
			[]string{"at least one of `a` and `b` is required"}, Config{Quotes: QuoteTick})
	})
	t.Run("quotes apply inside a custom message", func(t *testing.T) {
		fails(t, Rules{"age": `natural|error:custom "msg" here`}, Data{"age": -1},
			[]string{"custom `msg` here"}, Config{Quotes: QuoteTick})
	})
	t.Run("field: renames inside a nested object", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": "string|field:BEE"}}, Data{"a": Data{"b": 1}},
			[]string{"a.BEE must be string"})
	})
	t.Run("field: renames array elements", func(t *testing.T) {
		fails(t, Rules{"f": "arrayof:string|field:TAGS"}, Data{"f": []any{1}},
			[]string{"TAGS[0] must be string"})
	})
	t.Run("error: replaces a nested message", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": "string|error:nope"}}, Data{"a": Data{"b": 1}}, []string{"nope"})
	})
	t.Run("error: wins over field:", func(t *testing.T) {
		fails(t, Rules{"f": "string|field:F|error:E"}, Data{"f": 1}, []string{"E"})
	})
}

// --- string formats -------------------------------------------------------

func TestStringFormatCorners(t *testing.T) {
	accepted := []struct{ rule, value string }{
		{"email", "a+tag@b.com"},
		{"url", "https://a.com/p?q=1#z"},
		{"phone", "+91 98765 43210"},
		{"username", "abcdefgh"},
		{"username", "abcdefghijklmnopqrst"},
		{"date", "2024-06-01T10:00:00+05:30"},
		{"date", "20240601"},
		{"time", "12:30:45.123"},
		{"ip", "01.2.3.4"},
		{"uuid", "123E4567-E89B-12D3-A456-426655440000"},
		{"lower", "abc123"},
		{"upper", "AB C"},
	}
	for _, tc := range accepted {
		t.Run(tc.rule+" accepts "+tc.value, func(t *testing.T) {
			ok(t, Rules{"f": tc.rule}, Data{"f": tc.value})
		})
	}

	rejected := []struct{ rule, value, message string }{
		{"url", "ftp://a.com", "f must be a valid url"},
		{"domain", "sub.example.co.uk", "f must be a valid domain"},
		{"phone", "(022) 1234 5678", "f must be a valid phone"},
		{"username", "abcdefghijklmnopqrstu", "f must be a valid username"},
		{"alpha", "", "f must be a valid alpha"},
	}
	for _, tc := range rejected {
		t.Run(tc.rule+" rejects "+tc.value, func(t *testing.T) {
			fails(t, Rules{"f": tc.rule}, Data{"f": tc.value}, []string{tc.message})
		})
	}
}

// --- custom rules ---------------------------------------------------------

func TestCustomRuleContract(t *testing.T) {
	t.Run("a panic becomes a rule error", func(t *testing.T) {
		throws(t, Rules{"n": CustomRule(func(any, any) *RuleError { panic("boom") })}, Data{"n": 1})
	})
	t.Run("an empty message is a rule error", func(t *testing.T) {
		throws(t, Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Code: "C"}
		})}, Data{"n": 1})
	})
	t.Run("an empty code defaults to CUSTOM_RULE_FAILED", func(t *testing.T) {
		rules := Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Message: "m"}
		})}
		if got := codesOf(t, rules, Data{"n": 1}); len(got) != 1 || got[0] != CodeCustomRuleFail {
			t.Fatalf("expected CUSTOM_RULE_FAILED, got %v", got)
		}
	})
	t.Run("parent is the enclosing object", func(t *testing.T) {
		rules := Rules{"a": Rules{"b": CustomRule(func(_, parent any) *RuleError {
			m, _ := asMap(parent)
			n, _ := toNumber(m["c"])
			if n.Float == 1 {
				return nil
			}
			return &RuleError{Message: "need c", Code: "C"}
		})}}
		fails(t, rules, Data{"a": Data{"b": 1, "c": 2}}, []string{"need c"})
		ok(t, rules, Data{"a": Data{"b": 1, "c": 1}})
	})
	t.Run("parent is the element inside a tuple rule", func(t *testing.T) {
		rules := Rules{"items": []Rules{{"price": CustomRule(func(_, parent any) *RuleError {
			m, _ := asMap(parent)
			n, _ := toNumber(m["qty"])
			if n.Float > 0 {
				return nil
			}
			return &RuleError{Message: "need qty", Code: "Q"}
		})}}}
		fails(t, rules, Data{"items": []any{Data{"price": 1, "qty": 0}}}, []string{"need qty"})
	})
}

// --- presence and data guards ---------------------------------------------

func TestPresenceCombos(t *testing.T) {
	t.Run("optional and nullable together", func(t *testing.T) {
		ok(t, Rules{"f": "optional|nullable|string"}, Data{})
		ok(t, Rules{"f": "optional|nullable|string"}, Data{"f": nil})
	})
	t.Run("optional inside a nested object", func(t *testing.T) {
		ok(t, Rules{"a": Rules{"b": "optional|string"}}, Data{"a": Data{}})
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
	t.Run("empty rules pass any data", func(t *testing.T) {
		ok(t, Rules{}, Data{"a": 1})
	})
}

// --- group keys -----------------------------------------------------------

func TestGroupCodes(t *testing.T) {
	t.Run("$atleast emits ATLEAST_NOT_MET", func(t *testing.T) {
		rules := Rules{"a": "optional|string", "b": "optional|string", "$atleast": "a|b"}
		if got := codesOf(t, rules, Data{}); len(got) != 1 || got[0] != CodeAtleastNotMet {
			t.Fatalf("expected ATLEAST_NOT_MET, got %v", got)
		}
	})
	t.Run("$atmost emits ATMOST_EXCEEDED", func(t *testing.T) {
		rules := Rules{"a": "optional|string", "b": "optional|string", "$atmost": "a|b"}
		if got := codesOf(t, rules, Data{"a": "1", "b": "2"}); len(got) != 1 || got[0] != CodeAtmostExceeded {
			t.Fatalf("expected ATMOST_EXCEEDED, got %v", got)
		}
	})
	t.Run("$switch with no match emits NO_CASE_MATCHED", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{map[string]any{"case": "number", "then": "positive"}}}}
		if got := codesOf(t, rules, Data{"v": "x"}); len(got) != 1 || got[0] != CodeNoCaseMatched {
			t.Fatalf("expected NO_CASE_MATCHED, got %v", got)
		}
	})
	t.Run("strict emits UNEXPECTED_FIELD", func(t *testing.T) {
		result, _ := Validate(Rules{"a": "string"}, Data{"a": "x", "b": 1}, Config{Strict: true})
		if len(result.Details) != 1 || result.Details[0].Code != CodeUnexpectedField {
			t.Fatalf("expected UNEXPECTED_FIELD, got %#v", result.Details)
		}
	})
}
