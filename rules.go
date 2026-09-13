package validator

import "strings"

var bareRules = map[string]bool{
	"optional": true, "nullable": true,
	"string": true, "number": true, "boolean": true, "array": true,
	"object": true, "bigint": true,
	"email": true, "url": true, "domain": true, "name": true,
	"fullname": true, "username": true, "alpha": true, "alphanumeric": true,
	"phone": true, "phonecode": true, "objectid": true, "uuid": true,
	"date": true, "dateonly": true, "time": true, "lower": true,
	"upper": true, "ip": true,
	"int": true, "positive": true, "negative": true, "natural": true, "whole": true,
}

var argumentRules = map[string]bool{
	"equal": true, "size": true, "min": true, "max": true, "regex": true,
	"decimalsize": true, "decimalmin": true, "decimalmax": true,
	"enums": true, "field": true, "error": true,
}

const groupAtleast = "$atleast"
const groupAtmost = "$atmost"

func isGroupKey(k string) bool { return k == groupAtleast || k == groupAtmost }

// splitArg splits a token into its prefix and argument at the first colon.
// "min:5" yields ("min", "5", true); a bare rule yields ok=false.
func splitArg(token string) (prefix, arg string, ok bool) {
	i := strings.Index(token, ":")
	if i <= 0 {
		return "", "", false
	}
	return token[:i], token[i+1:], true
}

// assertKnownToken rejects an unknown rule so a typo surfaces on first run
// rather than being silently ignored.
func assertKnownToken(token, field string) error {
	if rest, found := strings.CutPrefix(token, "arrayof:"); found {
		if rest == "" {
			return invalidRule(field, "has an invalid rule: 'arrayof:' needs a rule after the colon")
		}
		return assertKnownToken(rest, field)
	}
	if bareRules[token] {
		return nil
	}
	if prefix, _, ok := splitArg(token); ok {
		if argumentRules[prefix] {
			return nil
		}
		return invalidRule(field, "has an unknown rule: '%s:'", prefix)
	}
	if token == "symbol" {
		return invalidRule(field, "has an unknown rule: 'symbol' has no Go equivalent and is not supported")
	}
	return invalidRule(field, "has an unknown rule: '%s'", token)
}

func assertValidRuleTokens(tokens []string, field string) error {
	for _, t := range tokens {
		if t == "" {
			continue
		}
		if err := assertKnownToken(t, field); err != nil {
			return err
		}
	}
	return nil
}

// tokenize splits a rule value into its tokens. A []string rule exists so a
// pattern or message may contain a literal "|".
func tokenize(value any) ([]string, bool) {
	switch v := value.(type) {
	case string:
		return strings.Split(v, "|"), true
	case []string:
		return v, true
	}
	return nil, false
}

func findPrefixed(tokens []string, prefix string) (string, bool) {
	for _, t := range tokens {
		if rest, found := strings.CutPrefix(t, prefix); found {
			return rest, true
		}
	}
	return "", false
}

// customField returns a field: override. It is suppressed for keys ending in
// an array index, matching npm.
func customField(tokens []string, key string) (string, bool) {
	if endsWithIndex(key) {
		return "", false
	}
	return findPrefixed(tokens, "field:")
}

func customError(tokens []string) (string, bool) {
	return findPrefixed(tokens, "error:")
}

// groupSize reads a size: token from a $atleast/$atmost rule, defaulting to 1.
func groupSize(tokens []string) int {
	if s, ok := findPrefixed(tokens, "size:"); ok {
		if n, err := parseInt(s); err == nil {
			return n
		}
	}
	return 1
}

func hasToken(tokens []string, want string) bool {
	for _, t := range tokens {
		if t == want {
			return true
		}
	}
	return false
}

// numericContext reports whether a preceding token puts the value in a
// numeric context, which selects the "numeric string" behaviour and messages.
func numericContext(previous []string) bool {
	for _, t := range previous {
		switch t {
		case "number", "positive", "negative", "int", "whole", "natural":
			return true
		}
	}
	return false
}

func hasStringContext(previous []string) bool {
	return hasToken(previous, "string") || hasToken(previous, "arrayof:string")
}
