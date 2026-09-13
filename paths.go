package validator

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	bracketRe   = regexp.MustCompile(`\[(-?\d*)(:?)(-?\d*)\]`)
	hasBracket  = regexp.MustCompile(`\[-?\d*:?-?\d*\]`)
	sliceRe     = regexp.MustCompile(`\[-?\d*:-?\d*\]`)
	trailingIdx = regexp.MustCompile(`\[-?\d+\]$`)
)

func hasIndexSyntax(key string) bool { return hasBracket.MatchString(key) }
func isSliceKey(key string) bool     { return sliceRe.MatchString(key) }
func endsWithIndex(key string) bool  { return trailingIdx.MatchString(key) }

func parseInt(s string) (int, error) { return strconv.Atoi(strings.TrimSpace(s)) }

// asSlice normalizes any Go slice or array to []any so heterogeneous data
// decoded from JSON and native slices behave alike.
func asSlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case nil:
		return nil, false
	}
	return reflectSlice(v)
}

// asMap normalizes a value to Data. Only map[string]any and Data are objects;
// everything else is not.
func asMap(v any) (Data, bool) {
	switch m := v.(type) {
	case Data:
		return m, true
	case map[string]any:
		return Data(m), true
	}
	return nil, false
}

// brokenChain reports a dotted path that could not be walked because an
// intermediate segment holds something other than an object. Segment is the
// path up to and including the offending value, so "a.b.c" against
// {"a": {"b": 5}} reports "a.b".
type brokenChain struct {
	Segment string
}

// lookup resolves a plain (non-indexed) key, walking dots. It returns whether
// the key was present, so an absent key stays distinguishable from a nil one.
//
// A key like "a.b.c" asserts that "a" and "a.b" are objects. When one of them
// holds a non-object instead, the walk stops and reports that segment: the
// value that is actually wrong, rather than the leaf that was never reachable.
// A nil intermediate counts as absent, not as a broken chain, so nullable
// data reads the same as missing data.
func lookup(data Data, key string) (value any, present bool, broken *brokenChain) {
	if !strings.Contains(key, ".") {
		v, ok := data[key]
		return v, ok, nil
	}

	parts := strings.Split(key, ".")
	cur := data
	for i := 0; i < len(parts)-1; i++ {
		next, ok := cur[parts[i]]
		if !ok || next == nil {
			return nil, false, nil
		}
		m, isMap := asMap(next)
		if !isMap {
			return nil, false, &brokenChain{Segment: strings.Join(parts[:i+1], ".")}
		}
		cur = m
	}
	v, ok := cur[parts[len(parts)-1]]
	return v, ok, nil
}

// lookupValue resolves a key when only the value matters, treating a broken
// chain as absence. Group keys use this: they count presence and have no
// channel for reporting a type error.
func lookupValue(data Data, key string) (any, bool) {
	v, present, _ := lookup(data, key)
	return v, present
}

// resolveIndexed resolves a key containing bracket syntax, returning the
// selected value and, for slices, a label per selected element.
func resolveIndexed(data Data, key string) (value any, labels []string, present bool) {
	var current any = data
	var sliceLabels []string
	prefix := ""

	for _, segment := range strings.Split(key, ".") {
		bracketStart := strings.Index(segment, "[")
		base := segment
		if bracketStart >= 0 {
			base = segment[:bracketStart]
		}

		if base != "" {
			if sliceLabels != nil {
				elems, ok := asSlice(current)
				if !ok {
					return nil, nil, false
				}
				var picked []any
				var kept []string
				for i, el := range elems {
					if m, isMap := asMap(el); isMap {
						v, found := m[base]
						if found {
							picked = append(picked, v)
							kept = append(kept, sliceLabels[i]+"."+base)
						}
					}
				}
				current, sliceLabels, prefix = picked, kept, ""
				continue
			}
			m, ok := asMap(current)
			if !ok {
				return nil, nil, false
			}
			v, found := m[base]
			if !found {
				return nil, nil, false
			}
			current = v
		}

		if prefix == "" {
			prefix = base
		} else {
			prefix = prefix + "." + base
		}

		if bracketStart < 0 {
			continue
		}

		for _, m := range bracketRe.FindAllStringSubmatch(segment[bracketStart:], -1) {
			from, colon, to := m[1], m[2], m[3]
			elems, ok := asSlice(current)
			if !ok {
				return nil, nil, false
			}
			if colon != "" {
				f, t := 0, len(elems)
				if from != "" {
					f, _ = parseInt(from)
				}
				if to != "" {
					t, _ = parseInt(to)
				}
				start := f
				if start < 0 {
					start = max(len(elems)+start, 0)
				} else {
					start = min(start, len(elems))
				}
				picked := sliceRange(elems, f, t)
				sliceLabels = make([]string, len(picked))
				for i := range picked {
					sliceLabels[i] = prefix + "[" + strconv.Itoa(start+i) + "]"
				}
				current = picked
			} else {
				idx, err := parseInt(from)
				if err != nil {
					return nil, nil, false
				}
				real := idx
				if real < 0 {
					real = len(elems) + real
				}
				if real < 0 || real >= len(elems) {
					return nil, nil, false
				}
				current = elems[real]
				prefix = prefix + "[" + from + "]"
			}
		}
	}

	return current, sliceLabels, true
}

// sliceRange mirrors JavaScript's Array.prototype.slice, including negative
// offsets and clamping.
func sliceRange(elems []any, from, to int) []any {
	n := len(elems)
	start, end := from, to
	if start < 0 {
		start = max(n+start, 0)
	} else {
		start = min(start, n)
	}
	if end < 0 {
		end = max(n+end, 0)
	} else {
		end = min(end, n)
	}
	if start >= end {
		return nil
	}
	return elems[start:end]
}

// resolve returns the value a rule key selects, and whether it was present.
// A broken dotted chain is reported separately so the caller can name the
// segment that is actually wrong.
func resolve(data Data, key string, cfg Config) (any, bool, *brokenChain) {
	if !cfg.DisableArrayIndexing && hasIndexSyntax(key) {
		v, _, ok := resolveIndexed(data, key)
		return v, ok, nil
	}
	return lookup(data, key)
}
