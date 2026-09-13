package validator

import "reflect"

// reflectSlice normalizes a native Go slice or array to []any, so callers may
// pass []string or []int without converting first. Strings are deliberately
// excluded: a string is not an array here.
func reflectSlice(v any) ([]any, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		for i := range out {
			out[i] = rv.Index(i).Interface()
		}
		return out, true
	}
	return nil, false
}

// isObject reports whether a value is an object rather than a scalar or array.
func isObject(v any) bool {
	_, ok := asMap(v)
	return ok
}

// isArray reports whether a value is an array.
func isArray(v any) bool {
	if v == nil {
		return false
	}
	if _, ok := v.(string); ok {
		return false
	}
	_, ok := asSlice(v)
	return ok
}
