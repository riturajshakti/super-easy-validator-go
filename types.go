package validator

import "fmt"

// Rules maps a field name to a rule value. A rule value may be:
//
//	string              a pipe-separated rule, e.g. "optional|email"
//	[]string            the same rules, for patterns containing a literal "|"
//	Rules               a nested object rule, or an operator node
//	[]Rules             a single-element tuple rule, for arrays of objects
//	CustomRule          a user-supplied function
//
// An operator node is a rule value whose single key is "$or", "$and" or
// "$switch":
//
//	Rules{"id": Rules{"$or": []any{"objectid", "uuid"}}}
//
// Because rules are plain maps, a whole rule tree can be decoded from JSON or
// built at runtime.
type Rules map[string]any

// Operator keys. An operator node carries exactly one of these and nothing
// else.
const (
	OpOr     = "$or"
	OpAnd    = "$and"
	OpSwitch = "$switch"
)

// Switch branch keys.
const (
	branchCase    = "case"
	branchThen    = "then"
	branchDefault = "default"
)

func isOperatorKey(k string) bool {
	return k == OpOr || k == OpAnd || k == OpSwitch
}

// Data is the value being validated, the shape encoding/json produces. A key
// that is absent and a key present with a nil value are distinct states:
// optional keys off the first, nullable off the second.
type Data map[string]any

// RuleError is returned by a CustomRule to report a failure. A nil return
// means the value passed.
type RuleError struct {
	Message string
	Code    ErrorCode
}

// CustomRule validates a single value. parent is the enclosing object, which
// makes cross-field checks possible. It is called even when the value is
// absent, so the function owns the decision about absence.
type CustomRule func(value, parent any) *RuleError

// Operator builds an operator node with typed fields, for callers who prefer
// that to writing the map literal. Exactly one field may be set. It converts
// to the same map form the engine uses, so these are equivalent:
//
//	Rules{"id": Rules{"$or": []any{"objectid", "uuid"}}}
//	Rules{"id": Operator{Or: []any{"objectid", "uuid"}}.Rules()}
type Operator struct {
	Or     []any
	And    []any
	Switch []SwitchBranch
}

// Rules converts the operator to its map form. Every field that is set is
// carried over, so an operator with more than one arm produces an invalid
// node and is reported as such rather than silently losing an arm.
func (o Operator) Rules() Rules {
	node := Rules{}
	if o.Or != nil {
		node[OpOr] = o.Or
	}
	if o.And != nil {
		node[OpAnd] = o.And
	}
	if o.Switch != nil {
		branches := make([]any, len(o.Switch))
		for i, b := range o.Switch {
			branches[i] = b.rules()
		}
		node[OpSwitch] = branches
	}
	return node
}

// SwitchBranch is one arm of a $switch. Then applies when Case passes;
// Default marks the arm used when no case matches.
type SwitchBranch struct {
	Case    any
	Then    any
	Default bool
}

func (b SwitchBranch) rules() map[string]any {
	m := map[string]any{branchCase: b.Case, branchThen: b.Then}
	if b.Default {
		m[branchDefault] = true
	}
	return m
}

// Detail is a single validation failure.
type Detail struct {
	Field   string
	Message string
	Code    ErrorCode
}

// Result is what Validate returns. Errors is nil when everything passed, so
// `if result.Errors != nil` is the idiom. Errors and Details are the same
// length and index-aligned.
type Result struct {
	Errors  []string
	Details []Detail
}

// Valid reports whether validation passed.
func (r Result) Valid() bool { return len(r.Errors) == 0 }

// QuoteStyle controls how field names are wrapped in error messages.
type QuoteStyle string

const (
	QuoteNone   QuoteStyle = "none"
	QuoteSingle QuoteStyle = "single-quotes"
	QuoteDouble QuoteStyle = "double-quotes"
	QuoteTick   QuoteStyle = "backtick"
)

// Config tunes validation. The zero value is the default behaviour.
type Config struct {
	// Quotes wraps field names in error messages. Default QuoteNone.
	Quotes QuoteStyle
	// Strict reports any data field that has no matching rule.
	Strict bool
	// DisableArrayIndexing treats keys like "c[0]" as literal names.
	DisableArrayIndexing bool
}

func (c Config) quoteChar() string {
	switch c.Quotes {
	case QuoteSingle:
		return "'"
	case QuoteDouble:
		return `"`
	case QuoteTick:
		return "`"
	default:
		return ""
	}
}

// InvalidRuleError reports a malformed rule. It is a programmer error, not a
// validation result, and is returned separately from Validate.
type InvalidRuleError struct {
	Field  string
	Reason string
}

func (e *InvalidRuleError) Error() string {
	return fmt.Sprintf("[super-easy-validator] '%s' %s", e.Field, e.Reason)
}

func invalidRule(field, format string, args ...any) *InvalidRuleError {
	return &InvalidRuleError{Field: field, Reason: fmt.Sprintf(format, args...)}
}
