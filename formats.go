package validator

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reEmail     = regexp.MustCompile(`^(?i)[A-Z0-9_'%=+!` + "`" + `#~$*?^{}&|-]+([.][A-Z0-9_'%=+!` + "`" + `#~$*?^{}&|-]+)*@[A-Z0-9-]+(\.[A-Z0-9-]+)+$`)
	reDomain    = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-_]{0,61}[a-zA-Z0-9]?\.([a-zA-Z]{1,6}|[a-zA-Z0-9-]{1,30}\.[a-zA-Z]{2,3})$`)
	reName      = regexp.MustCompile(`^\p{L}[\p{L}\p{M}]*\.?(?:[ '’\-]\p{L}[\p{L}\p{M}]*\.?)*$`)
	reFullname  = regexp.MustCompile(`^\p{L}[\p{L}\p{M}]*\.?(?:['’\-]\p{L}[\p{L}\p{M}]*\.?)*(?: \p{L}[\p{L}\p{M}]*\.?(?:['’\-]\p{L}[\p{L}\p{M}]*\.?)*)+$`)
	reAlpha     = regexp.MustCompile(`^[A-Za-z]+$`)
	reAlphanum  = regexp.MustCompile(`^[A-Za-z0-9]+$`)
	rePhone     = regexp.MustCompile(`^(?:\+\d{1,3}\s?)?(?:\(\d+\))?(?:\d+\s?)+\d{1,4}$`)
	rePhonecode = regexp.MustCompile(`^\+\d{1,3}$`)
	reObjectID  = regexp.MustCompile(`^[0-9a-fA-F]{24}$`)
	reUUID      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	reDate      = regexp.MustCompile(`^(\d{4})(?:-?(0[1-9]|1[0-2])(?:-?([12]\d|0[1-9]|3[01]))?|(?:-?([12]\d|0[1-9]|3[01]))?-(0[1-9]|1[0-2]))(?:[T ](\d{2}):([0-5]\d):([0-5]\d)(?:\.(\d{1,4}))?)?(?:Z|([+-])([01]\d|2[0-3])(?::?([0-5]\d))?)?$`)
	reDateonly  = regexp.MustCompile(`^(\d{4})-(0[1-9]|1[0-2])-([12]\d|0[1-9]|3[01])$`)
	reTime      = regexp.MustCompile(`^(?:[01]\d|2[0-3]):([0-5]\d)(?::([0-5]\d))?(?:\.\d{1,4})?$`)
	reLower     = regexp.MustCompile(`^[^A-Z]+$`)
	reUpper     = regexp.MustCompile(`^[^a-z]+$`)
	reIP        = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)

	// Forbidding consecutive "." or "_" is a rejection test rather than a
	// matching one, so it is applied separately in isUsername. RE2 has no
	// lookahead to express it inline.
	reUsernameShape = regexp.MustCompile(`^[^\W_][\w.]{6,18}[^\W_]$`)

	// A leading "www." is optional and otherwise unconstrained, so it needs no
	// special-casing in the pattern.
	reURL = regexp.MustCompile(`^(?:https?://[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]?\.[^\s]{2,}|www\.[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]?\.[^\s]{2,})$`)
)

var consecutiveSeparators = []string{"..", "._", "_.", "__"}

func isUsername(s string) bool {
	if !reUsernameShape.MatchString(s) {
		return false
	}
	for _, bad := range consecutiveSeparators {
		if strings.Contains(s, bad) {
			return false
		}
	}
	return true
}

// unsupportedRegexConstructs are backtracking features RE2 does not implement.
// No rewriting produces an equivalent, so a pattern using one is rejected.
var unsupportedRegexConstructs = []struct{ frag, name string }{
	{"(?=", "lookahead"},
	{"(?!", "negative lookahead"},
	{"(?<=", "lookbehind"},
	{"(?<!", "negative lookbehind"},
}

// compileUserRegex accepts a pattern in literal form, /pat/flags, and
// translates it to Go's inline-flag form. A pattern already in Go form passes
// through unchanged.
func compileUserRegex(pattern string) (*regexp.Regexp, error) {
	pat := pattern

	if strings.HasPrefix(pat, "/") {
		end := strings.LastIndex(pat, "/")
		if end == 0 {
			return nil, fmt.Errorf("has an unterminated regex literal: %s", pattern)
		}
		body, flags := pat[1:end], pat[end+1:]

		var goFlags string
		for _, f := range flags {
			switch f {
			case 'i', 'm', 's':
				goFlags += string(f)
			case 'g', 'y':
				// Iteration flags; meaningless for a full-string test.
			case 'u':
				// Go patterns are UTF-8 aware by default.
			default:
				return nil, fmt.Errorf("has an unsupported regex flag '%c'", f)
			}
		}
		pat = body
		if goFlags != "" {
			pat = "(?" + goFlags + ")" + body
		}
	}

	for _, u := range unsupportedRegexConstructs {
		if strings.Contains(pat, u.frag) {
			return nil, fmt.Errorf("uses %s '%s', which Go's regexp engine does not support", u.name, u.frag)
		}
	}
	if regexpBackreference.MatchString(pat) {
		return nil, fmt.Errorf("uses a backreference, which Go's regexp engine does not support")
	}

	re, err := regexp.Compile(pat)
	if err != nil {
		return nil, fmt.Errorf("has an invalid regex: %v", err)
	}
	return re, nil
}

var regexpBackreference = regexp.MustCompile(`\\[1-9]`)
