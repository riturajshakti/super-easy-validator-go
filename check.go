package validator

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"
)

// checker accumulates failures for one field. start records how many errors
// the shared slice already held, so failed() asks "did THIS field fail",
// not "has anything failed so far".
type checker struct {
	key    string
	prefix string
	tokens []string
	errs   *[]Detail
	start  int
}

func newChecker(key, prefix string, tokens []string, errs *[]Detail) *checker {
	return &checker{key: key, prefix: prefix, tokens: tokens, errs: errs, start: len(*errs)}
}

// label is the field name shown in messages, honouring a field: override.
func (c *checker) label() string {
	name := c.key
	if custom, ok := customField(c.tokens, c.key); ok {
		name = custom
	}
	if c.prefix != "" {
		return c.prefix + "." + name
	}
	return name
}

// fail records a failure, honouring an error: override while keeping the code.
func (c *checker) fail(code ErrorCode, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if custom, ok := customError(c.tokens); ok {
		msg = custom
	}
	*c.errs = append(*c.errs, Detail{Field: c.label(), Message: msg, Code: code})
}

func (c *checker) failed() bool { return len(*c.errs) > c.start }

// checkSingle applies a token list to one value. It stops at the first
// failure, so a field reports at most one error.
func checkSingle(key, prefix string, value any, present bool, tokens []string, errs *[]Detail) {
	c := newChecker(key, prefix, tokens, errs)

	// optional keys off absence; nullable keys off an explicit null. The two
	// are distinct, which is why Data is a map rather than a struct.
	if hasToken(tokens, "optional") && !present {
		return
	}
	if hasToken(tokens, "nullable") && present && value == nil {
		return
	}

	for i, token := range tokens {
		previous := tokens[:i]

		switch {
		case token == "optional" || token == "nullable" || token == "":
			continue

		case isDataTypeToken(token):
			checkDataType(c, value, present, token, previous)

		case isStringFormatToken(token):
			checkStringFormat(c, value, present, token, previous)

		case isNumberTypeToken(token):
			checkNumberType(c, value, present, token, previous)

		case strings.HasPrefix(token, "arrayof:"):
			checkArrayOf(c, value, present, token, previous)

		default:
			if prefix, arg, ok := splitArg(token); ok && argumentRules[prefix] {
				if prefix == "field" || prefix == "error" {
					continue
				}
				checkConstraint(c, value, present, prefix, arg, previous)
			}
		}

		if c.failed() {
			return
		}
	}
}

func isDataTypeToken(t string) bool {
	switch t {
	case "string", "number", "boolean", "array", "object", "bigint":
		return true
	}
	return false
}

func isStringFormatToken(t string) bool {
	switch t {
	case "email", "url", "domain", "name", "fullname", "username", "alpha",
		"alphanumeric", "phone", "phonecode", "objectid", "uuid", "date",
		"dateonly", "time", "lower", "upper", "ip":
		return true
	}
	return false
}

func isNumberTypeToken(t string) bool {
	switch t {
	case "int", "positive", "negative", "natural", "whole":
		return true
	}
	return false
}

// requirePresent reports the shared "is required" failure for a value that is
// absent or null.
func requirePresent(c *checker, value any, present bool) bool {
	if !present || value == nil {
		c.fail(CodeRequired, `"%s" is required`, c.label())
		return false
	}
	return true
}

func checkDataType(c *checker, value any, present bool, dataType string, previous []string) {
	if !requirePresent(c, value, present) {
		return
	}
	hasString := hasStringContext(previous)

	switch dataType {
	case "string":
		if _, ok := value.(string); !ok {
			c.fail(CodeNotString, `"%s" must be string`, c.label())
		}

	case "number":
		if hasString {
			s, ok := value.(string)
			if !ok {
				c.fail(CodeNotString, `"%s" must be string`, c.label())
				return
			}
			if _, valid := numberFromString(s); !valid {
				c.fail(CodeNotNumericString, `"%s" must be a valid numeric string`, c.label())
			}
			return
		}
		if !isNumericKind(value) {
			c.fail(CodeNotNumber, `"%s" must be a valid number`, c.label())
		}

	case "boolean":
		if hasString {
			s, ok := value.(string)
			if !ok || (s != "true" && s != "false") {
				c.fail(CodeNotBooleanString, `"%s" must be a valid boolean string`, c.label())
			}
			return
		}
		if _, ok := value.(bool); !ok {
			c.fail(CodeNotBoolean, `"%s" must be a valid boolean`, c.label())
		}

	case "array":
		if !isArray(value) {
			c.fail(CodeNotArray, `"%s" must be an array`, c.label())
		}

	case "object":
		if !isObject(value) {
			c.fail(CodeNotObject, `"%s" must be an object`, c.label())
		}

	case "bigint":
		if !isBigint(value) {
			c.fail(CodeNotBigint, `"%s" must be bigint`, c.label())
		}
	}
}

// isBigint accepts *big.Int and json.Number, so an integer too large for
// float64 survives a JSON round trip - which it cannot in the npm package.
func isBigint(value any) bool {
	switch n := value.(type) {
	case *big.Int:
		return n != nil
	case json.Number:
		_, err := n.Int64()
		if err == nil {
			return true
		}
		_, ok := new(big.Int).SetString(n.String(), 10)
		return ok
	}
	return false
}

func checkStringFormat(c *checker, value any, present bool, format string, previous []string) {
	if !requirePresent(c, value, present) {
		return
	}
	checkDataType(c, value, present, "string", previous)
	if c.failed() {
		return
	}
	s := value.(string)

	switch format {
	case "email":
		if !reEmail.MatchString(s) {
			c.fail(CodeNotEmail, `"%s" must be a valid email`, c.label())
		}
	case "url":
		if !reURL.MatchString(s) {
			c.fail(CodeNotURL, `"%s" must be a valid url`, c.label())
		}
	case "domain":
		if !reDomain.MatchString(s) {
			c.fail(CodeNotDomain, `"%s" must be a valid domain`, c.label())
		}
	case "name":
		if !reName.MatchString(s) {
			c.fail(CodeNotName, `"%s" must be a valid name`, c.label())
		}
	case "fullname":
		if !reFullname.MatchString(s) {
			c.fail(CodeNotFullname, `"%s" must be a valid fullname`, c.label())
		}
	case "username":
		if !isUsername(s) {
			c.fail(CodeNotUsername, `"%s" must be a valid username`, c.label())
		}
	case "alpha":
		if !reAlpha.MatchString(s) {
			c.fail(CodeNotAlpha, `"%s" must be a valid alpha`, c.label())
		}
	case "alphanumeric":
		if !reAlphanum.MatchString(s) {
			c.fail(CodeNotAlphanum, `"%s" must be a valid alphanumeric`, c.label())
		}
	case "phone":
		if !rePhone.MatchString(s) {
			c.fail(CodeNotPhone, `"%s" must be a valid phone`, c.label())
		}
	case "phonecode":
		if !rePhonecode.MatchString(s) {
			c.fail(CodeNotPhonecode, `"%s" must be a valid phone code`, c.label())
		}
	case "objectid":
		if !reObjectID.MatchString(s) {
			c.fail(CodeNotObjectID, `"%s" must be a valid object id`, c.label())
		}
	case "uuid":
		if !reUUID.MatchString(s) {
			c.fail(CodeNotUUID, `"%s" must be a valid uuid`, c.label())
		}
	case "date":
		if !reDate.MatchString(s) {
			c.fail(CodeNotDate, `"%s" must be a valid date`, c.label())
		}
	case "dateonly":
		if !reDateonly.MatchString(s) {
			c.fail(CodeNotDateonly, `"%s" must be a valid date`, c.label())
		}
	case "time":
		if !reTime.MatchString(s) {
			c.fail(CodeNotTime, `"%s" must be a valid time`, c.label())
		}
	case "lower":
		if !reLower.MatchString(s) {
			c.fail(CodeNotLowercase, `"%s" must not contains upper case letters`, c.label())
		}
	case "upper":
		if !reUpper.MatchString(s) {
			c.fail(CodeNotUppercase, `"%s" must not contains lower case letters`, c.label())
		}
	case "ip":
		if !reIP.MatchString(s) {
			c.fail(CodeNotIP, `"%s" must be a valid IP address`, c.label())
		}
	}
}

func checkNumberType(c *checker, value any, present bool, numType string, previous []string) {
	if !requirePresent(c, value, present) {
		return
	}
	checkDataType(c, value, present, "number", previous)
	if c.failed() {
		return
	}

	hasString := hasStringContext(previous)
	var n number
	if hasString {
		n, _ = numberFromString(value.(string))
	} else {
		n, _ = toNumber(value)
	}

	suffix := "number"
	if hasString {
		suffix = "numeric string"
	}

	switch numType {
	case "int":
		if !n.IsInt {
			word := "integer"
			if hasString {
				word = "integer string"
			}
			c.fail(CodeNotInteger, `"%s" must be a valid %s`, c.label(), word)
		}
	case "positive":
		if !(n.Float > 0) {
			c.fail(CodeNotPositive, `"%s" must be a valid positive %s`, c.label(), suffix)
		}
	case "negative":
		if !(n.Float < 0) {
			c.fail(CodeNotNegative, `"%s" must be a valid negative %s`, c.label(), suffix)
		}
	case "natural":
		if !n.IsInt || n.Float <= 0 {
			c.fail(CodeNotNatural, `"%s" must be a valid natural %s`, c.label(), suffix)
		}
	case "whole":
		if !n.IsInt || n.Float < 0 {
			c.fail(CodeNotWhole, `"%s" must be a valid whole %s`, c.label(), suffix)
		}
	}
}

func checkConstraint(c *checker, value any, present bool, kind, arg string, previous []string) {
	if !requirePresent(c, value, present) {
		return
	}

	str, isStr := value.(string)
	num, isNum := toNumber(value)
	boolean, isBool := value.(bool)
	numeric := numericContext(previous)
	arr, isArr := asSlice(value)
	if _, isString := value.(string); isString {
		isArr = false
	}

	switch kind {
	case "equal":
		switch {
		case isStr && str != arg:
			c.fail(CodeNotEqual, `"%s" must be equal to %s`, c.label(), arg)
		case isNum && !isStr:
			if want, ok := numberFromString(arg); !ok || want.Float != num.Float {
				c.fail(CodeNotEqual, `"%s" must be equal to %s`, c.label(), arg)
			}
		case isBool:
			if (arg == "true" && !boolean) || (arg == "false" && boolean) {
				c.fail(CodeNotEqual, `"%s" must be equal to %s`, c.label(), arg)
			}
		}

	case "size":
		size, err := parseInt(arg)
		if err != nil {
			return
		}
		switch {
		case isStr && !numeric:
			if len([]rune(str)) != size {
				c.fail(CodeLengthMismatch, `"%s" must have length %d`, c.label(), size)
			}
		case isArr:
			if len(arr) != size {
				c.fail(CodeLengthMismatch, `"%s" must have length %d`, c.label(), size)
			}
		case isNum:
			if num.digits() != size {
				c.fail(CodeDigitsMismatch, `"%s" must have %d digits`, c.label(), size)
			}
		}

	case "min", "max":
		checkBound(c, kind, arg, value, str, isStr, num, isNum, arr, isArr, numeric)

	case "regex":
		re, err := compileUserRegex(arg)
		if err != nil {
			// Surfaced by validateRules before any data is examined.
			return
		}
		if !isStr {
			c.fail(CodeNotString, `"%s" must be of type string`, c.label())
			return
		}
		if !re.MatchString(str) {
			c.fail(CodeRegexMismatch, `"%s" is invalid`, c.label())
		}

	case "decimalsize", "decimalmin", "decimalmax":
		checkDecimals(c, kind, arg, value, str, isStr, num, isNum)

	case "enums":
		options := strings.Split(arg, ",")
		switch {
		case isStr && !numeric:
			if !contains(options, str) {
				c.fail(CodeEnumMismatch, `"%s" is invalid`, c.label())
			}
		case isNum:
			matched := false
			for _, o := range options {
				if want, ok := numberFromString(o); ok && want.Float == num.Float {
					matched = true
					break
				}
			}
			if !matched {
				c.fail(CodeEnumMismatch, `"%s" is invalid`, c.label())
			}
		case isBool:
			want := "false"
			if boolean {
				want = "true"
			}
			if !contains(options, want) {
				c.fail(CodeEnumMismatch, `"%s" is invalid`, c.label())
			}
		}
	}
}

// checkBound applies min:/max:, which are overloaded across string length,
// array length, numeric value and date comparison.
func checkBound(c *checker, kind, arg string, value any, str string, isStr bool, num number, isNum bool, arr []any, isArr bool, numeric bool) {
	isMin := kind == "min"

	bound, numericBound := numberFromString(arg)
	if !numericBound {
		// A non-numeric argument is a date bound.
		limit, err := parseDate(arg)
		if err != nil {
			return
		}
		actual, err := parseDate(fmt.Sprint(value))
		if err != nil {
			return
		}
		if isMin && actual.Before(limit) {
			c.fail(CodeDateTooEarly, `"%s" must be at least %s`, c.label(), limit.UTC().Format(isoFormat))
		}
		if !isMin && actual.After(limit) {
			c.fail(CodeDateTooLate, `"%s" must be at most %s`, c.label(), limit.UTC().Format(isoFormat))
		}
		return
	}

	switch {
	case isStr && !numeric:
		n := len([]rune(str))
		if isMin && n < int(bound.Float) {
			c.fail(CodeTooShort, `"%s" must have length of at least %s`, c.label(), arg)
		}
		if !isMin && n > int(bound.Float) {
			c.fail(CodeTooLong, `"%s" must have length of at most %s`, c.label(), arg)
		}
	case isArr:
		n := len(arr)
		if isMin && n < int(bound.Float) {
			c.fail(CodeTooShort, `"%s" must have length of at least %s`, c.label(), arg)
		}
		if !isMin && n > int(bound.Float) {
			c.fail(CodeTooLong, `"%s" must have length of at most %s`, c.label(), arg)
		}
	case isNum:
		if isMin && num.Float < bound.Float {
			c.fail(CodeTooSmall, `"%s" must be at least %s`, c.label(), arg)
		}
		if !isMin && num.Float > bound.Float {
			c.fail(CodeTooLarge, `"%s" must be at most %s`, c.label(), arg)
		}
	}
}

const isoFormat = "2006-01-02T15:04:05.000Z"

var dateLayouts = []string{
	time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05",
	"2006-01-02 15:04:05", "2006-01-02", "2006-01", "2006",
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range dateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("not a date: %s", s)
}

func checkDecimals(c *checker, kind, arg string, value any, str string, isStr bool, num number, isNum bool) {
	want, err := parseInt(arg)
	if err != nil {
		return
	}

	var n number
	switch {
	case isStr:
		parsed, ok := numberFromString(str)
		if !ok {
			c.fail(CodeNotNumericString, `"%s" must be a valid numeric string`, c.label())
			return
		}
		n = parsed
	case isNum:
		if num.isNaN() {
			// npm's message reads "a value number"; kept verbatim so ported
			// expectations match.
			c.fail(CodeNotANumber, `"%s" must be a value number`, c.label())
			return
		}
		n = num
	default:
		return
	}

	digits, hasPoint := n.decimals()

	switch kind {
	case "decimalsize":
		if !hasPoint {
			if want > 0 {
				c.fail(CodeDecimalSizeMismatch, `"%s" must have %d decimal places`, c.label(), want)
			}
			return
		}
		if digits != want {
			c.fail(CodeDecimalSizeMismatch, `"%s" must have %d decimal places`, c.label(), want)
		}
	case "decimalmin":
		if !hasPoint {
			if want > 0 {
				c.fail(CodeDecimalTooFew, `"%s" must have at least %d decimal places`, c.label(), want)
			}
			return
		}
		if digits < want {
			c.fail(CodeDecimalTooFew, `"%s" must have at least %d decimal places`, c.label(), want)
		}
	case "decimalmax":
		if !hasPoint {
			return
		}
		if digits > want {
			c.fail(CodeDecimalTooMany, `"%s" must have at most %d decimal places`, c.label(), want)
		}
	}
}

// checkArrayOf applies the inner rule to every element, labelling failures
// with the element's index.
func checkArrayOf(c *checker, value any, present bool, token string, previous []string) {
	inner := strings.TrimPrefix(token, "arrayof:")

	elems, ok := asSlice(value)
	if !ok || !isArray(value) {
		c.fail(CodeNotArray, `"%s" must be an array`, c.label())
		return
	}

	elementOptional := hasToken(c.tokens, "arrayof:optional")
	elementNullable := hasToken(c.tokens, "arrayof:nullable")

	if inner == "optional" || inner == "nullable" {
		return
	}

	for i, el := range elems {
		missing := el == nil
		if (elementOptional && missing) || (elementNullable && missing) {
			continue
		}

		elementKey := fmt.Sprintf("%s[%d]", c.label(), i)
		// Element errors carry their own label, so the parent prefix and the
		// field:/error: overrides do not apply again.
		var elErrs []Detail
		sub := newChecker(elementKey, "", c.tokens, &elErrs)

		switch {
		case isDataTypeToken(inner):
			checkDataType(sub, el, !missing, inner, previous)
		case isStringFormatToken(inner):
			checkStringFormat(sub, el, !missing, inner, previous)
		case isNumberTypeToken(inner):
			checkNumberType(sub, el, !missing, inner, previous)
		case strings.HasPrefix(inner, "arrayof:"):
			checkArrayOf(sub, el, !missing, inner, previous)
		default:
			if prefix, arg, ok := splitArg(inner); ok && argumentRules[prefix] {
				checkConstraint(sub, el, !missing, prefix, arg, previous)
			}
		}

		*c.errs = append(*c.errs, elErrs...)
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

var _ = strconv.Itoa
