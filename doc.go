// Package validator validates data against rules written as plain strings.
//
// Rules are strings, not builder chains or struct tags:
//
//	rules := validator.Rules{
//		"name":  "fullname",
//		"email": "email",
//		"age":   "optional|natural|min:18",
//		"role":  "enums:admin,user,guest",
//	}
//
//	result, err := validator.Validate(rules, data)
//	if err != nil {
//		// a malformed rule: a programmer error, not a validation failure
//	}
//	if !result.Valid() {
//		fmt.Println(result.Errors)
//	}
//
// # Data
//
// Data is map[string]any, the shape encoding/json produces. An absent key and
// a key present with a nil value are distinct states: optional keys off the
// first and nullable off the second. Structs are not accepted, because
// encoding/json collapses null and missing into the same nil pointer and the
// distinction would be lost. Marshal a struct to a map first.
//
// # Numbers
//
// Callers never reason about bit widths. Integer rules (int, natural, whole)
// work in integers and number and the decimal rules work in floats; every Go
// numeric kind normalizes to that pair, so int, int8..int64, uint..uint64,
// float32 and float64 all behave identically. Integer-ness is decided by
// value rather than static type, so a JSON 7 (a float64) satisfies natural.
//
// bigint accepts *big.Int and json.Number, so an integer too large for
// float64 survives a JSON round trip.
//
// # Regular expressions
//
// Patterns may be written in literal form, /pattern/flags, and are translated
// automatically:
//
//	regex:/^[A-Z0-9]{128}$/i   ->   (?i)^[A-Z0-9]{128}$
//
// Go's regexp is RE2, which has no backtracking, so lookahead, lookbehind and
// backreferences are reported as rule errors rather than silently never
// matching.
//
// # Error ordering
//
// Go randomizes map iteration, so error order within one rules level is
// unspecified. Sort by Detail.Field when deterministic output is needed.
package validator
