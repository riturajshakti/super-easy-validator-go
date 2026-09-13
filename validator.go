package validator

import (
	"fmt"
	"sort"
	"strings"
)

// Validate checks data against rules. It returns a Result whose Errors is nil
// when everything passed.
//
// A malformed rule is a programmer error, not a validation failure, and is
// returned as the second value (an *InvalidRuleError) with a zero Result.
func Validate(rules Rules, data Data, config ...Config) (Result, error) {
	cfg := Config{}
	if len(config) > 0 {
		cfg = config[0]
	}

	if err := validateRules(rules, ""); err != nil {
		return Result{}, err
	}

	var details []Detail
	if err := walk(rules, data, cfg, "", &details); err != nil {
		return Result{}, err
	}

	return finish(details, cfg), nil
}

// MustValidate is Validate for callers who treat a malformed rule as fatal.
func MustValidate(rules Rules, data Data, config ...Config) Result {
	result, err := Validate(rules, data, config...)
	if err != nil {
		panic(err)
	}
	return result
}

// finish applies the quote style and collapses duplicate messages, keeping
// the first occurrence's code.
func finish(details []Detail, cfg Config) Result {
	quote := cfg.quoteChar()
	seen := make(map[string]bool, len(details))

	var out []Detail
	for _, d := range details {
		msg := strings.ReplaceAll(d.Message, `"`, quote)
		if seen[msg] {
			continue
		}
		seen[msg] = true
		out = append(out, Detail{Field: d.Field, Message: msg, Code: d.Code})
	}

	if len(out) == 0 {
		return Result{}
	}

	errors := make([]string, len(out))
	for i, d := range out {
		errors[i] = d.Message
	}
	return Result{Errors: errors, Details: out}
}

// validateRules walks the rule tree once, before any data is examined, so a
// typo or a malformed operator surfaces immediately.
func validateRules(rules Rules, prefix string) error {
	for key, value := range rules {
		field := key
		if prefix != "" {
			field = prefix + "." + key
		}

		if isGroupKey(key) {
			if _, ok := tokenize(value); !ok {
				if _, isList := value.([]string); !isList {
					return invalidRule(field, "must be a rule string or a list of rule strings")
				}
			}
			continue
		}

		if err := validateRuleValue(value, field); err != nil {
			return err
		}
	}
	return nil
}

func validateRuleValue(value any, field string) error {
	switch v := value.(type) {
	case nil:
		return invalidRule(field, "has an invalid rule: it must not be nil")

	case string:
		if err := assertValidRuleTokens(strings.Split(v, "|"), field); err != nil {
			return err
		}
		return checkUserRegexes(strings.Split(v, "|"), field)

	case []string:
		if err := assertValidRuleTokens(v, field); err != nil {
			return err
		}
		return checkUserRegexes(v, field)

	case CustomRule:
		return nil

	case func(any, any) *RuleError:
		return nil

	case Operator:
		return validateRuleValue(v.Rules(), field)

	case *Operator:
		if v == nil {
			return invalidRule(field, "has an invalid rule: the operator must not be nil")
		}
		return validateRuleValue(v.Rules(), field)

	case Rules:
		if isOperatorNode(v) {
			return validateOperatorNode(v, field)
		}
		return validateRules(v, field)

	case map[string]any:
		if isOperatorNode(Rules(v)) {
			return validateOperatorNode(Rules(v), field)
		}
		return validateRules(Rules(v), field)

	case []Rules:
		if len(v) != 1 {
			return invalidRule(field, "has an invalid rule: a tuple rule must have exactly one element")
		}
		return validateRules(v[0], field)

	case []any:
		// A nested tuple rule: [][]Rules spelled as []any{[]Rules{...}}.
		if len(v) != 1 {
			return invalidRule(field, "has an invalid rule: a tuple rule must have exactly one element")
		}
		return validateRuleValue(v[0], field)

	default:
		return invalidRule(field, "has an invalid rule: unsupported rule type %T", value)
	}
}

// checkUserRegexes compiles every regex: argument up front so an unsupported
// construct is reported as a rule error rather than a silent non-match.
func checkUserRegexes(tokens []string, field string) error {
	for _, t := range tokens {
		if arg, found := strings.CutPrefix(t, "regex:"); found {
			if _, err := compileUserRegex(arg); err != nil {
				return invalidRule(field, "%s", err.Error())
			}
		}
	}
	return nil
}

// isOperatorNode reports whether a rule value is an operator node rather than
// a nested object rule.
func isOperatorNode(value Rules) bool {
	for k := range value {
		if isOperatorKey(k) {
			return true
		}
	}
	return false
}

// operatorOf returns the operator key and its branches, having already been
// validated.
func operatorOf(node Rules) (string, []any) {
	for _, op := range []string{OpOr, OpAnd, OpSwitch} {
		if raw, ok := node[op]; ok {
			branches, _ := raw.([]any)
			return op, branches
		}
	}
	return "", nil
}

func validateOperatorNode(node Rules, field string) error {
	var operators []string
	for _, op := range []string{OpOr, OpAnd, OpSwitch} {
		if _, ok := node[op]; ok {
			operators = append(operators, op)
		}
	}
	if len(operators) > 1 {
		return invalidRule(field, "cannot combine %s in one object", strings.Join(operators, " and "))
	}

	operator := operators[0]
	var extras []string
	for k := range node {
		if k != operator {
			extras = append(extras, k)
		}
	}
	if len(extras) > 0 {
		sort.Strings(extras)
		return invalidRule(field, "cannot mix '%s' with other keys (found: %s); wrap them in a branch instead",
			operator, strings.Join(extras, ", "))
	}

	branches, ok := node[operator].([]any)
	if !ok {
		return invalidRule(field, "has an invalid rule: '%s' must be an array of branches", operator)
	}
	if len(branches) == 0 {
		return invalidRule(field, "has an invalid rule: '%s' needs at least one branch", operator)
	}

	if operator == OpSwitch {
		return validateSwitchBranches(branches, field)
	}

	for _, branch := range branches {
		if err := validateRuleValue(branch, field); err != nil {
			return err
		}
		if operator == OpOr && onlyModifiers(branch) {
			return invalidRule(field, "has an invalid rule: a '$or' branch cannot be only 'optional' or 'nullable', because it would accept any value. Use '$and' to make the field optional.")
		}
	}
	return nil
}

func validateSwitchBranches(branches []any, field string) error {
	defaults := 0

	for _, raw := range branches {
		branch, ok := asMap(raw)
		if !ok {
			return invalidRule(field, "has an invalid rule: every '$switch' branch must be an object with 'case' and 'then'")
		}

		var extras []string
		for k := range branch {
			if k != branchCase && k != branchThen && k != branchDefault {
				extras = append(extras, k)
			}
		}
		if len(extras) > 0 {
			sort.Strings(extras)
			return invalidRule(field, "has an invalid rule: a '$switch' branch accepts only 'case', 'then' and 'default' (found: %s)",
				strings.Join(extras, ", "))
		}

		caseRule, hasCase := branch[branchCase]
		if !hasCase || caseRule == nil {
			return invalidRule(field, "has an invalid rule: a '$switch' branch needs a 'case'")
		}
		thenRule, hasThen := branch[branchThen]
		if !hasThen || thenRule == nil {
			return invalidRule(field, "has an invalid rule: a '$switch' branch needs a 'then'")
		}
		if raw, ok := branch[branchDefault]; ok {
			flag, isBool := raw.(bool)
			if !isBool {
				return invalidRule(field, "has an invalid rule: '$switch' 'default' must be a boolean")
			}
			if flag {
				defaults++
			}
		}

		if err := validateRuleValue(caseRule, field); err != nil {
			return err
		}
		if err := validateRuleValue(thenRule, field); err != nil {
			return err
		}
	}

	if defaults > 1 {
		return invalidRule(field, "has an invalid rule: only one '$switch' branch may be marked 'default'")
	}
	return nil
}

func onlyModifiers(branch any) bool {
	tokens, ok := tokenize(branch)
	if !ok {
		return false
	}
	for _, t := range tokens {
		if t != "" && t != "optional" && t != "nullable" {
			return false
		}
	}
	return true
}

// walk applies every rule in one level to the data at that level.
func walk(rules Rules, data Data, cfg Config, prefix string, out *[]Detail) error {
	if data == nil {
		name := "data"
		if prefix != "" {
			name = prefix
		}
		*out = append(*out, Detail{Field: name, Message: fmt.Sprintf(`"%s" is required`, name), Code: CodeDataRequired})
		return nil
	}

	for key, value := range rules {
		if isGroupKey(key) {
			checkGroup(key, value, data, prefix, out)
			continue
		}

		field := key
		if prefix != "" {
			field = prefix + "." + key
		}

		if err := applyRule(key, field, value, data, cfg, prefix, out); err != nil {
			return err
		}
	}

	if cfg.Strict {
		strictCheck(rules, data, prefix, out)
	}
	return nil
}

// applyRule dispatches one rule value against the data it selects.
func applyRule(key, field string, value any, data Data, cfg Config, prefix string, out *[]Detail) error {
	switch v := value.(type) {
	case CustomRule:
		return runCustom(v, key, field, data, cfg, out)

	case func(any, any) *RuleError:
		return runCustom(CustomRule(v), key, field, data, cfg, out)

	case Operator:
		return applyRule(key, field, v.Rules(), data, cfg, prefix, out)

	case *Operator:
		return applyRule(key, field, v.Rules(), data, cfg, prefix, out)

	case Rules:
		if isOperatorNode(v) {
			return applyOperator(v, key, field, prefix, data, cfg, out)
		}
		return applyNested(v, key, field, data, cfg, out)

	case map[string]any:
		if isOperatorNode(Rules(v)) {
			return applyOperator(Rules(v), key, field, prefix, data, cfg, out)
		}
		return applyNested(Rules(v), key, field, data, cfg, out)

	case []Rules:
		return applyTuple(v[0], key, field, data, cfg, out)

	case []any:
		return applyTuple(v[0], key, field, data, cfg, out)

	case string, []string:
		tokens, _ := tokenize(v)
		return applyTokens(tokens, key, field, data, cfg, prefix, out)
	}
	return nil
}

// applyTokens handles the common case: a token list against one value, or
// against each element a slice key selects.
func applyTokens(tokens []string, key, field string, data Data, cfg Config, prefix string, out *[]Detail) error {
	indexing := !cfg.DisableArrayIndexing && hasIndexSyntax(key)

	if indexing && isSliceKey(key) {
		selected, labels, ok := resolveIndexed(data, key)
		if !ok {
			*out = append(*out, Detail{Field: key, Message: fmt.Sprintf(`"%s" must be an array`, key), Code: CodeNotArray})
			return nil
		}
		elems, isSlice := asSlice(selected)
		if !isSlice {
			*out = append(*out, Detail{Field: key, Message: fmt.Sprintf(`"%s" must be an array`, key), Code: CodeNotArray})
			return nil
		}
		for i, el := range elems {
			label := fmt.Sprintf("%s[%d]", key, i)
			if i < len(labels) {
				label = labels[i]
			}
			checkSingle(label, prefix, el, el != nil, tokens, out)
		}
		return nil
	}

	value, present, broken := resolve(data, key, cfg)
	if broken != nil {
		// A key like "a.b.c" asserts that "a" and "a.b" are objects. Report
		// the segment that is not, rather than the leaf it hid. optional does
		// not excuse this: the value is present, it is the wrong type.
		reportBrokenChain(broken, prefix, out)
		return nil
	}
	checkSingle(key, prefix, value, present, tokens, out)
	return nil
}

// reportBrokenChain records the failing segment of a dotted path.
func reportBrokenChain(broken *brokenChain, prefix string, out *[]Detail) {
	label := broken.Segment
	if prefix != "" {
		label = prefix + "." + label
	}
	*out = append(*out, Detail{
		Field:   label,
		Message: fmt.Sprintf(`"%s" must be of type object`, label),
		Code:    CodeNotObject,
	})
}

// applyNested validates a nested object rule.
func applyNested(sub Rules, key, field string, data Data, cfg Config, out *[]Detail) error {
	value, present := data[key]
	if !present || value == nil {
		*out = append(*out, Detail{Field: field, Message: fmt.Sprintf(`"%s" is required`, field), Code: CodeRequired})
		return nil
	}
	inner, ok := asMap(value)
	if !ok {
		*out = append(*out, Detail{Field: field, Message: fmt.Sprintf(`"%s" must be of type object`, field), Code: CodeNotObject})
		return nil
	}
	return walk(sub, inner, cfg, field, out)
}

// applyTuple validates an array of objects against a single-element rule.
// applyTuple validates an array against a single-element tuple rule. The
// element rule may itself be a tuple, so [][]Rules validates an array of
// arrays of objects at any depth.
func applyTuple(sub any, key, field string, data Data, cfg Config, out *[]Detail) error {
	value, present := data[key]
	if !present || value == nil {
		*out = append(*out, Detail{Field: field, Message: fmt.Sprintf(`"%s" is required`, field), Code: CodeRequired})
		return nil
	}
	return applyTupleValue(sub, field, value, cfg, out)
}

func applyTupleValue(sub any, field string, value any, cfg Config, out *[]Detail) error {
	elems, ok := asSlice(value)
	if !ok || !isArray(value) {
		*out = append(*out, Detail{Field: field, Message: fmt.Sprintf(`"%s" must be of type array`, field), Code: CodeNotArray})
		return nil
	}

	inner, nested := tupleElement(sub)

	for i, el := range elems {
		path := fmt.Sprintf("%s[%d]", field, i)

		if nested {
			if err := applyTupleValue(inner, path, el, cfg, out); err != nil {
				return err
			}
			continue
		}

		if el == nil {
			*out = append(*out, Detail{Field: path, Message: fmt.Sprintf(`"%s" is required`, path), Code: CodeRequired})
			continue
		}
		obj, isMap := asMap(el)
		if !isMap {
			*out = append(*out, Detail{Field: path, Message: fmt.Sprintf(`"%s" must be of type object/array`, path), Code: CodeNotObject})
			continue
		}
		if err := walk(sub.(Rules), obj, cfg, path, out); err != nil {
			return err
		}
	}
	return nil
}

// tupleElement unwraps one level of tuple nesting, reporting whether the
// element rule is itself a tuple.
func tupleElement(sub any) (any, bool) {
	switch v := sub.(type) {
	case []Rules:
		if len(v) == 1 {
			return v[0], true
		}
	case []any:
		if len(v) == 1 {
			return v[0], true
		}
	}
	return sub, false
}

func runCustom(fn CustomRule, key, field string, data Data, cfg Config, out *[]Detail) error {
	// A custom rule owns its own decision about absence, so a broken chain
	// reaches it as a nil value rather than as an error.
	value, _, _ := resolve(data, key, cfg)
	result, err := callCustom(fn, value, any(data), field)
	if err != nil {
		return err
	}
	if result == nil {
		return nil
	}
	if result.Message == "" {
		return invalidRule(field, "has an invalid custom rule: the returned RuleError needs a Message")
	}
	code := result.Code
	if code == "" {
		code = CodeCustomRuleFail
	}
	*out = append(*out, Detail{Field: field, Message: result.Message, Code: code})
	return nil
}

// branchErrors runs one branch against a value, reusing the main engine so a
// branch may be any rule shape.
func branchErrors(branch any, key, field string, value any, present bool, parent Data, cfg Config) ([]Detail, error) {
	shortKey := key
	if i := strings.LastIndex(field, "."); i >= 0 {
		shortKey = field[i+1:]
	}
	parentName := ""
	if i := strings.LastIndex(field, "."); i >= 0 {
		parentName = field[:i]
	}

	if fn, ok := asCustomRule(branch); ok {
		result, err := callCustom(fn, value, any(parent), field)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return nil, nil
		}
		code := result.Code
		if code == "" {
			code = CodeCustomRuleFail
		}
		return []Detail{{Field: field, Message: result.Message, Code: code}}, nil
	}

	wrapper := Rules{shortKey: branch}
	inner := Data{}
	if present {
		inner[shortKey] = value
	}

	branchCfg := cfg
	if hasIndexSyntax(shortKey) {
		branchCfg.DisableArrayIndexing = true
	}

	var details []Detail
	if err := walk(wrapper, inner, branchCfg, parentName, &details); err != nil {
		return nil, err
	}
	return details, nil
}

// callCustom runs a user function, converting a panic into a rule error so a
// mistake in a custom rule is reported like any other malformed rule rather
// than taking down the caller.
func callCustom(fn CustomRule, value, parent any, field string) (result *RuleError, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = invalidRule(field, "custom rule threw an error: %v", r)
		}
	}()
	return fn(value, parent), nil
}

func asCustomRule(branch any) (CustomRule, bool) {
	switch f := branch.(type) {
	case CustomRule:
		return f, true
	case func(any, any) *RuleError:
		return CustomRule(f), true
	}
	return nil, false
}

func applyOperator(node Rules, key, field, prefix string, data Data, cfg Config, out *[]Detail) error {
	value, present, broken := resolve(data, key, cfg)
	if broken != nil {
		// Reported before any branch runs, so $or does not score branches
		// against a value the path never reached.
		reportBrokenChain(broken, prefix, out)
		return nil
	}
	operator, branches := operatorOf(node)

	switch operator {
	case OpSwitch:
		return applySwitch(branches, key, field, value, present, data, cfg, out)
	case OpAnd:
		return applyAnd(branches, key, field, value, present, data, cfg, out)
	default:
		return applyOr(branches, key, field, value, present, data, cfg, out)
	}
}

// modifierBranch reports whether a branch carries optional/nullable, which
// lets $and and $or make a nested object or array optional.
func modifierBranch(branch any, modifier string) bool {
	tokens, ok := tokenize(branch)
	if !ok {
		return false
	}
	return hasToken(tokens, modifier)
}

func anyBranchHas(branches []any, modifier string) bool {
	for _, b := range branches {
		if modifierBranch(b, modifier) {
			return true
		}
	}
	return false
}

func applyAnd(branches []any, key, field string, value any, present bool, data Data, cfg Config, out *[]Detail) error {
	if !present && anyBranchHas(branches, "optional") {
		return nil
	}
	if present && value == nil && anyBranchHas(branches, "nullable") {
		return nil
	}

	// Plain object branches are merged into one rule set, so a key declared
	// in any branch counts as declared under strict mode.
	merged := mergedObjectBranches(branches)
	mergedDone := false

	for _, branch := range branches {
		if onlyModifiers(branch) {
			continue
		}
		if merged != nil && isPlainObjectBranch(branch) {
			if mergedDone {
				continue
			}
			mergedDone = true
			details, err := branchErrors(merged, key, field, value, present, data, cfg)
			if err != nil {
				return err
			}
			*out = append(*out, details...)
			continue
		}
		details, err := branchErrors(branch, key, field, value, present, data, cfg)
		if err != nil {
			return err
		}
		*out = append(*out, details...)
	}
	return nil
}

// isPlainObjectBranch reports whether a branch is a nested object rule rather
// than an operator node.
func isPlainObjectBranch(branch any) bool {
	switch b := branch.(type) {
	case Rules:
		return !isOperatorNode(b)
	case map[string]any:
		return !isOperatorNode(Rules(b))
	}
	return false
}

// mergedObjectBranches combines every plain object branch into one rule set,
// returning nil when there are fewer than two to merge.
func mergedObjectBranches(branches []any) Rules {
	var objects []Rules
	for _, branch := range branches {
		if !isPlainObjectBranch(branch) {
			continue
		}
		switch b := branch.(type) {
		case Rules:
			objects = append(objects, b)
		case map[string]any:
			objects = append(objects, Rules(b))
		}
	}
	if len(objects) < 2 {
		return nil
	}
	merged := Rules{}
	for _, o := range objects {
		for k, v := range o {
			merged[k] = v
		}
	}
	return merged
}

func applySwitch(branches []any, key, field string, value any, present bool, data Data, cfg Config, out *[]Detail) error {
	var fallback any
	var hasFallback bool

	for _, raw := range branches {
		branch, _ := asMap(raw)
		if flag, ok := branch[branchDefault].(bool); ok && flag {
			fallback, hasFallback = branch[branchThen], true
		}
		caseErrors, err := branchErrors(branch[branchCase], key, field, value, present, data, cfg)
		if err != nil {
			return err
		}
		if len(caseErrors) == 0 {
			details, err := branchErrors(branch[branchThen], key, field, value, present, data, cfg)
			if err != nil {
				return err
			}
			*out = append(*out, details...)
			return nil
		}
	}

	if hasFallback {
		details, err := branchErrors(fallback, key, field, value, present, data, cfg)
		if err != nil {
			return err
		}
		*out = append(*out, details...)
		return nil
	}

	*out = append(*out, Detail{Field: field, Message: fmt.Sprintf(`"%s" does not match any case`, field), Code: CodeNoCaseMatched})
	return nil
}

// applyOr reports the branch that best fits the value, so a near-miss gets a
// specific message rather than a vague "is invalid".
func applyOr(branches []any, key, field string, value any, present bool, data Data, cfg Config, out *[]Detail) error {
	if !present && anyBranchHas(branches, "optional") {
		return nil
	}
	if present && value == nil && anyBranchHas(branches, "nullable") {
		return nil
	}

	type attempt struct {
		errors []Detail
		typed  bool
	}

	attempts := make([]attempt, 0, len(branches))
	for _, branch := range branches {
		details, err := branchErrors(branch, key, field, value, present, data, cfg)
		if err != nil {
			return err
		}
		if len(details) == 0 {
			return nil
		}
		attempts = append(attempts, attempt{errors: details, typed: branchTypeMatches(branch, value)})
	}
	if len(attempts) == 0 {
		return nil
	}

	// A branch failing only on strict-mode extras is reported as-is.
	for _, a := range attempts {
		if allUnexpected(a.errors) {
			*out = append(*out, a.errors...)
			return nil
		}
	}

	pool := make([]attempt, 0, len(attempts))
	for _, a := range attempts {
		if a.typed {
			pool = append(pool, a)
		}
	}
	if len(pool) == 0 {
		pool = attempts
	}

	best := pool[0]
	for _, a := range pool[1:] {
		if len(a.errors) < len(best.errors) {
			best = a
		}
	}

	for _, d := range best.errors {
		if d.Code != CodeUnexpectedField {
			*out = append(*out, d)
		}
	}
	return nil
}

func allUnexpected(details []Detail) bool {
	for _, d := range details {
		if d.Code != CodeUnexpectedField {
			return false
		}
	}
	return len(details) > 0
}

// branchTypeMatches reports whether a branch is shaped for this value's type,
// which is how $or prefers the branch the caller meant.
func branchTypeMatches(branch any, value any) bool {
	if _, ok := asCustomRule(branch); ok {
		return true
	}

	switch b := branch.(type) {
	case Operator, *Operator:
		return true
	case Rules:
		if isOperatorNode(b) {
			return true
		}
		return isObject(value)
	case map[string]any:
		if isOperatorNode(Rules(b)) {
			return true
		}
		return isObject(value)
	case []Rules, []any:
		return isArray(value)
	case []string:
		return tokensMatchType(b, value)
	case string:
		return tokensMatchType(strings.Split(b, "|"), value)
	}
	return false
}

func tokensMatchType(tokens []string, value any) bool {
	for _, t := range tokens {
		if strings.HasPrefix(t, "arrayof:") {
			return isArray(value)
		}
	}
	switch {
	case hasToken(tokens, "object"):
		return isObject(value)
	case hasToken(tokens, "array"):
		return isArray(value)
	case hasToken(tokens, "string"):
		_, ok := value.(string)
		return ok
	case hasToken(tokens, "number"):
		return isNumericKind(value)
	case hasToken(tokens, "boolean"):
		_, ok := value.(bool)
		return ok
	}
	return !isObject(value) && !isArray(value)
}

// checkGroup implements $atleast and $atmost, which count how many of the
// named fields are present.
func checkGroup(kind string, value any, data Data, prefix string, out *[]Detail) {
	var groups [][]string
	switch v := value.(type) {
	case string:
		groups = [][]string{strings.Split(v, "|")}
	case []string:
		for _, g := range v {
			groups = append(groups, strings.Split(g, "|"))
		}
	default:
		return
	}

	for _, tokens := range groups {
		size := groupSize(tokens)
		count := 0
		var names []string

		for _, t := range tokens {
			if strings.Contains(t, ":") {
				continue
			}
			names = append(names, t)
			if v, ok := lookupValue(data, t); ok && v != nil {
				count++
			}
		}
		if len(names) == 0 {
			continue
		}

		if kind == groupAtleast && count >= size {
			continue
		}
		if kind == groupAtmost && count <= size {
			continue
		}

		labelled := make([]string, len(names))
		for i, n := range names {
			if prefix != "" {
				labelled[i] = prefix + "." + n
			} else {
				labelled[i] = n
			}
		}

		message := groupMessage(kind, labelled, size)
		if custom, ok := customError(tokens); ok {
			message = custom
		}
		code := CodeAtleastNotMet
		if kind == groupAtmost {
			code = CodeAtmostExceeded
		}
		*out = append(*out, Detail{Field: labelled[0], Message: message, Code: code})
	}
}

func groupMessage(kind string, names []string, size int) string {
	sizeWord := fmt.Sprint(size)
	if size == 1 {
		sizeWord = "one"
	}

	last := names[len(names)-1]
	leading := make([]string, 0, len(names))
	for _, n := range names[:len(names)-1] {
		leading = append(leading, `"`+n+`"`)
	}
	joined := strings.Join(leading, ", ")

	andWord := ""
	if len(names) > 1 {
		andWord = "and"
	}

	if kind == groupAtleast {
		isAre := "are"
		if size == 1 {
			isAre = "is"
		}
		return strings.Join(nonEmpty([]string{
			"at least " + sizeWord + " of", joined, andWord, `"` + last + `"`, isAre, "required",
		}), " ")
	}
	return strings.Join(nonEmpty([]string{
		"at most " + sizeWord + " of", joined, andWord, `"` + last + `"`, "can be given",
	}), " ")
}

func nonEmpty(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// strictCheck reports data fields that have no matching rule.
func strictCheck(rules Rules, data Data, prefix string, out *[]Detail) {
	allowed := make(map[string]bool, len(rules))
	for key := range rules {
		if isGroupKey(key) || strings.Contains(key, ".") {
			continue
		}
		base := key
		if i := strings.Index(key, "["); i > 0 {
			base = key[:i]
		}
		allowed[base] = true
	}

	for key := range data {
		if strings.Contains(key, ".") || allowed[key] {
			continue
		}
		label := key
		if prefix != "" {
			label = prefix + "." + key
		}
		*out = append(*out, Detail{Field: label, Message: fmt.Sprintf(`"%s" is not required`, label), Code: CodeUnexpectedField})
	}
}
