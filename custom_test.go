package validator

import "testing"

// Custom rules: the return contract, the parent parameter, every position a
// function may occupy, and how it interacts with the rest of the library.

func evenRule(v, _ any) *RuleError {
	n, ok := toNumber(v)
	if ok && n.IsInt && n.Int%2 == 0 {
		return nil
	}
	return &RuleError{Message: "must be even", Code: "NOT_EVEN"}
}

// --- basics ---------------------------------------------------------------

func TestCustomBasics(t *testing.T) {
	t.Run("nil means the value passes", func(t *testing.T) {
		ok(t, Rules{"n": CustomRule(evenRule)}, Data{"n": 4})
	})
	t.Run("a RuleError becomes an error", func(t *testing.T) {
		fails(t, Rules{"n": CustomRule(evenRule)}, Data{"n": 3}, []string{"must be even"})
	})
	t.Run("the function is called even when the value is absent", func(t *testing.T) {
		fails(t, Rules{"n": CustomRule(evenRule)}, Data{}, []string{"must be even"})
	})
	t.Run("an absent value arrives as nil", func(t *testing.T) {
		seen := "unset"
		Validate(Rules{"n": CustomRule(func(v, _ any) *RuleError {
			if v == nil {
				seen = "nil"
			} else {
				seen = "non-nil"
			}
			return nil
		})}, Data{})
		if seen != "nil" {
			t.Fatalf("expected nil for an absent value, got %s", seen)
		}
	})
	t.Run("a null value arrives as nil", func(t *testing.T) {
		seen := "unset"
		Validate(Rules{"n": CustomRule(func(v, _ any) *RuleError {
			if v == nil {
				seen = "nil"
			} else {
				seen = "non-nil"
			}
			return nil
		})}, Data{"n": nil})
		if seen != "nil" {
			t.Fatalf("expected nil for a null value, got %s", seen)
		}
	})
	t.Run("the developer can accept absence explicitly", func(t *testing.T) {
		ok(t, Rules{"n": CustomRule(func(v, _ any) *RuleError {
			if v == nil {
				return nil
			}
			return evenRule(v, nil)
		})}, Data{})
	})
	t.Run("the function is called exactly once per field", func(t *testing.T) {
		calls := 0
		Validate(Rules{"n": CustomRule(func(any, any) *RuleError {
			calls++
			return nil
		})}, Data{"n": 1})
		if calls != 1 {
			t.Fatalf("expected exactly one call, got %d", calls)
		}
	})
}

// --- the parent parameter -------------------------------------------------

func TestCustomParent(t *testing.T) {
	t.Run("parent is the enclosing object", func(t *testing.T) {
		var seen Data
		Validate(Rules{"o": Rules{"n": CustomRule(func(_, p any) *RuleError {
			seen, _ = asMap(p)
			return nil
		})}}, Data{"o": Data{"n": 1, "m": 2}})
		if len(seen) != 2 || seen["n"] != 1 || seen["m"] != 2 {
			t.Fatalf("expected the enclosing object, got %#v", seen)
		}
	})
	t.Run("parent inside a tuple element is that element", func(t *testing.T) {
		var seen Data
		Validate(Rules{"u": []Rules{{"n": CustomRule(func(_, p any) *RuleError {
			seen, _ = asMap(p)
			return nil
		})}}}, Data{"u": []any{Data{"n": 1, "k": "a"}}})
		if len(seen) != 2 || seen["k"] != "a" {
			t.Fatalf("expected the tuple element, got %#v", seen)
		}
	})
	t.Run("cross-field comparison", func(t *testing.T) {
		rules := Rules{
			"password": "string|min:8",
			"confirmPassword": CustomRule(func(v, p any) *RuleError {
				m, _ := asMap(p)
				if v == m["password"] {
					return nil
				}
				return &RuleError{Message: "passwords must match", Code: "PASSWORD_MISMATCH"}
			}),
		}
		ok(t, rules, Data{"password": "longenough", "confirmPassword": "longenough"})
		fails(t, rules, Data{"password": "longenough", "confirmPassword": "different"},
			[]string{"passwords must match"})
	})
	t.Run("conditional requirement based on a sibling", func(t *testing.T) {
		rules := Rules{
			"type": "enums:personal,business",
			"taxId": CustomRule(func(v, p any) *RuleError {
				m, _ := asMap(p)
				if m["type"] == "business" && v == nil {
					return &RuleError{Message: "taxId required for business", Code: "REQ"}
				}
				return nil
			}),
		}
		ok(t, rules, Data{"type": "personal"})
		fails(t, rules, Data{"type": "business"}, []string{"taxId required for business"})
	})
	t.Run("nested parent access", func(t *testing.T) {
		rules := Rules{"o": Rules{"n": CustomRule(func(_, p any) *RuleError {
			m, _ := asMap(p)
			n, _ := toNumber(m["m"])
			if n.Float == 1 {
				return nil
			}
			return &RuleError{Message: "m must be 1", Code: "C"}
		})}}
		fails(t, rules, Data{"o": Data{"n": 0, "m": 2}}, []string{"m must be 1"})
	})
}

// --- every position a custom rule can occupy ------------------------------

func TestCustomPositions(t *testing.T) {
	t.Run("inside a nested object", func(t *testing.T) {
		fails(t, Rules{"o": Rules{"n": CustomRule(evenRule)}}, Data{"o": Data{"n": 3}},
			[]string{"must be even"})
	})
	t.Run("two levels deep", func(t *testing.T) {
		fails(t, Rules{"a": Rules{"b": Rules{"n": CustomRule(evenRule)}}},
			Data{"a": Data{"b": Data{"n": 3}}}, []string{"must be even"})
	})
	t.Run("inside a tuple element", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"n": CustomRule(evenRule)}}},
			Data{"u": []any{Data{"n": 3}}}, []string{"must be even"})
	})
	t.Run("runs for every tuple element", func(t *testing.T) {
		fails(t, Rules{"u": []Rules{{"n": CustomRule(evenRule)}}},
			Data{"u": []any{Data{"n": 2}, Data{"n": 3}}}, []string{"must be even"})
	})
	t.Run("on a dotted path", func(t *testing.T) {
		fails(t, Rules{"a.n": CustomRule(evenRule)}, Data{"a": Data{"n": 3}},
			[]string{"must be even"})
	})
	t.Run("on an indexed key", func(t *testing.T) {
		fails(t, Rules{"c[0]": CustomRule(evenRule)}, Data{"c": []any{3}},
			[]string{"must be even"})
	})
	t.Run("on a negative index", func(t *testing.T) {
		fails(t, Rules{"c[-1]": CustomRule(evenRule)}, Data{"c": []any{2, 3}},
			[]string{"must be even"})
	})
	t.Run("on a slice key", func(t *testing.T) {
		fails(t, Rules{"c[0:2]": CustomRule(evenRule)}, Data{"c": []any{3, 5}},
			[]string{"must be even"})
	})
	t.Run("a plain func literal needs no conversion", func(t *testing.T) {
		fails(t, Rules{"n": func(v, _ any) *RuleError { return evenRule(v, nil) }},
			Data{"n": 3}, []string{"must be even"})
	})
}

// --- inside operators -----------------------------------------------------

func TestCustomInOperators(t *testing.T) {
	t.Run("$or: the function branch can satisfy it", func(t *testing.T) {
		ok(t, Rules{"n": Rules{"$or": []any{CustomRule(evenRule), "string"}}}, Data{"n": 4})
	})
	t.Run("$or: another branch can satisfy it", func(t *testing.T) {
		ok(t, Rules{"n": Rules{"$or": []any{CustomRule(evenRule), "string"}}}, Data{"n": "hi"})
	})
	t.Run("$or: reports the function when no branch passes", func(t *testing.T) {
		fails(t, Rules{"n": Rules{"$or": []any{CustomRule(evenRule), "string"}}}, Data{"n": 3},
			[]string{"must be even"})
	})
	t.Run("$or: a custom rule as the only branch", func(t *testing.T) {
		fails(t, Rules{"n": Rules{"$or": []any{CustomRule(evenRule)}}}, Data{"n": 3},
			[]string{"must be even"})
	})
	t.Run("$and: runs alongside string rules", func(t *testing.T) {
		rules := Rules{"n": Rules{"$and": []any{"natural", CustomRule(evenRule)}}}
		ok(t, rules, Data{"n": 4})
		fails(t, rules, Data{"n": 3}, []string{"must be even"})
	})
	t.Run("$and: optional skips when absent", func(t *testing.T) {
		ok(t, Rules{"n": Rules{"$and": []any{"optional", CustomRule(evenRule)}}}, Data{})
	})
	t.Run("$and: optional still runs when present", func(t *testing.T) {
		fails(t, Rules{"n": Rules{"$and": []any{"optional", CustomRule(evenRule)}}}, Data{"n": 3},
			[]string{"must be even"})
	})
	t.Run("$and: two custom rules both report", func(t *testing.T) {
		tooSmall := CustomRule(func(v, _ any) *RuleError {
			n, _ := toNumber(v)
			if n.Float > 10 {
				return nil
			}
			return &RuleError{Message: "too small", Code: "TS"}
		})
		fails(t, Rules{"n": Rules{"$and": []any{CustomRule(evenRule), tooSmall}}}, Data{"n": 3},
			[]string{"must be even", "too small"})
	})
	t.Run("$and: the parent is available", func(t *testing.T) {
		rules := Rules{"a": "natural", "b": Rules{"$and": []any{"natural", CustomRule(func(v, p any) *RuleError {
			m, _ := asMap(p)
			av, _ := toNumber(m["a"])
			bv, _ := toNumber(v)
			if bv.Float > av.Float {
				return nil
			}
			return &RuleError{Message: "b must exceed a", Code: "ORDER"}
		})}}}
		ok(t, rules, Data{"a": 1, "b": 2})
		fails(t, rules, Data{"a": 5, "b": 2}, []string{"b must exceed a"})
	})
	t.Run("$switch: a function as the case", func(t *testing.T) {
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
	t.Run("$switch: a function as the then", func(t *testing.T) {
		rules := Rules{"v": Rules{"$switch": []any{map[string]any{
			"case": "number",
			"then": CustomRule(func(x, _ any) *RuleError {
				n, _ := toNumber(x)
				if n.Float > 0 {
					return nil
				}
				return &RuleError{Message: "neg", Code: "N"}
			})}}}}
		fails(t, rules, Data{"v": -5}, []string{"neg"})
	})
}

// --- integration with the rest of the library -----------------------------

func TestCustomIntegration(t *testing.T) {
	t.Run("the custom code appears in details", func(t *testing.T) {
		result, err := Validate(Rules{"n": CustomRule(evenRule)}, Data{"n": 3})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Details) != 1 || result.Details[0].Code != "NOT_EVEN" ||
			result.Details[0].Message != "must be even" {
			t.Fatalf("unexpected details: %#v", result.Details)
		}
	})
	t.Run("errors and details stay aligned", func(t *testing.T) {
		result, _ := Validate(Rules{"a": CustomRule(evenRule), "b": "natural"}, Data{"a": 1, "b": -1})
		if len(result.Errors) != len(result.Details) {
			t.Fatalf("length mismatch")
		}
		for i := range result.Errors {
			if result.Errors[i] != result.Details[i].Message {
				t.Fatalf("index %d misaligned", i)
			}
		}
	})
	t.Run("any code string is accepted", func(t *testing.T) {
		rules := Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Message: "m", Code: "MY_OWN_CODE"}
		})}
		if got := codesOf(t, rules, Data{"n": 1}); len(got) != 1 || got[0] != "MY_OWN_CODE" {
			t.Fatalf("expected MY_OWN_CODE, got %v", got)
		}
	})
	t.Run("a built-in code may be reused", func(t *testing.T) {
		rules := Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Message: "m", Code: CodeRequired}
		})}
		if got := codesOf(t, rules, Data{"n": 1}); len(got) != 1 || got[0] != CodeRequired {
			t.Fatalf("expected REQUIRED, got %v", got)
		}
	})
	t.Run("identical messages are de-duplicated", func(t *testing.T) {
		fails(t, Rules{"a": CustomRule(evenRule), "b": CustomRule(evenRule)},
			Data{"a": 1, "b": 3}, []string{"must be even"})
	})
	t.Run("strict counts the key as declared", func(t *testing.T) {
		ok(t, Rules{"n": CustomRule(evenRule)}, Data{"n": 4}, Config{Strict: true})
	})
	t.Run("strict still reports an extra key", func(t *testing.T) {
		fails(t, Rules{"n": CustomRule(evenRule)}, Data{"n": 4, "z": 1},
			[]string{"z is not required"}, Config{Strict: true})
	})
	t.Run("mixes with ordinary string rules", func(t *testing.T) {
		fails(t, Rules{"a": "natural", "n": CustomRule(evenRule)}, Data{"a": -1, "n": 3},
			[]string{"a must be a valid natural number", "must be even"})
	})
}

// --- the return contract --------------------------------------------------

func TestCustomReturnContract(t *testing.T) {
	t.Run("an empty message is a rule error", func(t *testing.T) {
		throws(t, Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Code: "C"}
		})}, Data{"n": 1})
	})
	t.Run("the rule error names the field", func(t *testing.T) {
		_, err := Validate(Rules{"deep": Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Code: "C"}
		})}}, Data{"deep": Data{"n": 1}})
		if err == nil {
			t.Fatal("expected a rule error")
		}
		if e, isRuleErr := err.(*InvalidRuleError); !isRuleErr || e.Field != "deep.n" {
			t.Fatalf("expected the field path, got %v", err)
		}
	})
	t.Run("an omitted code defaults to CUSTOM_RULE_FAILED", func(t *testing.T) {
		rules := Rules{"n": CustomRule(func(any, any) *RuleError {
			return &RuleError{Message: "m"}
		})}
		if got := codesOf(t, rules, Data{"n": 1}); len(got) != 1 || got[0] != CodeCustomRuleFail {
			t.Fatalf("expected CUSTOM_RULE_FAILED, got %v", got)
		}
	})
	t.Run("a panic becomes a rule error naming the field", func(t *testing.T) {
		_, err := Validate(Rules{"n": CustomRule(func(any, any) *RuleError {
			panic("boom")
		})}, Data{"n": 1})
		if err == nil {
			t.Fatal("expected a rule error")
		}
		if e, isRuleErr := err.(*InvalidRuleError); !isRuleErr || e.Field != "n" {
			t.Fatalf("expected the field name, got %v", err)
		}
	})
	t.Run("a panic inside an operator branch is recovered", func(t *testing.T) {
		throws(t, Rules{"n": Rules{"$and": []any{"natural", CustomRule(func(any, any) *RuleError {
			panic("boom")
		})}}}, Data{"n": 1})
	})
}

// --- realistic uses -------------------------------------------------------

func TestCustomRealistic(t *testing.T) {
	t.Run("a total that must equal the sum of its parts", func(t *testing.T) {
		rules := Rules{
			"items": "array|arrayof:number",
			"total": CustomRule(func(v, p any) *RuleError {
				m, _ := asMap(p)
				elems, _ := asSlice(m["items"])
				sum := 0.0
				for _, e := range elems {
					n, _ := toNumber(e)
					sum += n.Float
				}
				got, _ := toNumber(v)
				if got.Float == sum {
					return nil
				}
				return &RuleError{Message: "total does not match items", Code: "BAD_TOTAL"}
			}),
		}
		ok(t, rules, Data{"items": []any{1, 2, 3}, "total": 6})
		fails(t, rules, Data{"items": []any{1, 2, 3}, "total": 7},
			[]string{"total does not match items"})
	})
	t.Run("a domain rule the built-ins do not cover", func(t *testing.T) {
		luhn := CustomRule(func(v, _ any) *RuleError {
			s, _ := v.(string)
			sum, alt := 0, false
			for i := len(s) - 1; i >= 0; i-- {
				d := int(s[i] - '0')
				if alt {
					d *= 2
					if d > 9 {
						d -= 9
					}
				}
				sum += d
				alt = !alt
			}
			if sum%10 == 0 {
				return nil
			}
			return &RuleError{Message: "invalid card number", Code: "BAD_CARD"}
		})
		ok(t, Rules{"card": luhn}, Data{"card": "4539578763621486"})
		fails(t, Rules{"card": luhn}, Data{"card": "1234567812345678"},
			[]string{"invalid card number"})
	})
	t.Run("combined with built-in rules through $and", func(t *testing.T) {
		rules := Rules{"age": Rules{"$and": []any{"natural|min:18", CustomRule(func(v, _ any) *RuleError {
			n, _ := toNumber(v)
			if n.IsInt {
				return nil
			}
			return &RuleError{Message: "no fractions", Code: "FRAC"}
		})}}}
		ok(t, rules, Data{"age": 30})
		if errs := errorsOf(t, rules, Data{"age": 10}); len(errs) == 0 {
			t.Fatal("expected an error for an under-age value")
		}
	})
}
