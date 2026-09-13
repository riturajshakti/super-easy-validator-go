package validator

import (
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
)

// number is a numeric value normalized away from Go's eleven numeric kinds.
// Callers never see int32/float32 distinctions: integer rules use Int, and
// number and the decimal rules use Float.
type number struct {
	Int   int64
	Float float64
	IsInt bool
	// Text is the value's original textual form when one exists (a numeric
	// string or json.Number). Decimal rules use it so trailing zeroes in
	// "10.50" survive, which they cannot through a float.
	Text string
}

func isNumericKind(v any) bool {
	_, ok := toNumber(v)
	return ok
}

// toNumber widens any Go numeric kind to the single int/float pair. Widening
// happens once, at the boundary, so a float32 cannot leak its precision
// artifacts into digit counting later.
func toNumber(v any) (number, bool) {
	switch n := v.(type) {
	case json.Number:
		return numberFromString(n.String())
	case *big.Int:
		if n == nil {
			return number{}, false
		}
		if n.IsInt64() {
			i := n.Int64()
			return number{Int: i, Float: float64(i), IsInt: true, Text: n.String()}, true
		}
		f, _ := new(big.Float).SetInt(n).Float64()
		return number{Float: f, IsInt: true, Text: n.String()}, true
	case bool:
		return number{}, false
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i := rv.Int()
		return number{Int: i, Float: float64(i), IsInt: true}, true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u := rv.Uint()
		return number{Int: int64(u), Float: float64(u), IsInt: true}, true
	case reflect.Float32:
		f := rv.Float()
		return number{Int: int64(f), Float: f, IsInt: isIntegral(f), Text: strconv.FormatFloat(f, 'f', -1, 32)}, true
	case reflect.Float64:
		f := rv.Float()
		return number{Int: int64(f), Float: f, IsInt: isIntegral(f)}, true
	}
	return number{}, false
}

func isIntegral(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0) && f == math.Trunc(f)
}

// numberFromString parses a numeric string, keeping the original text so
// decimal rules can count the digits the user actually wrote.
func numberFromString(s string) (number, bool) {
	t := strings.TrimSpace(s)
	if t == "" {
		return number{}, false
	}
	f, err := strconv.ParseFloat(t, 64)
	if err != nil {
		return number{}, false
	}
	n := number{Float: f, Int: int64(f), IsInt: isIntegral(f) && !strings.ContainsAny(t, ".eE"), Text: t}
	if i, err := strconv.ParseInt(t, 10, 64); err == nil {
		n.Int, n.IsInt = i, true
	}
	return n, true
}

// text renders a number the way digit-counting rules read it, preferring the
// original spelling when there was one.
func (n number) text() string {
	if n.Text != "" {
		return n.Text
	}
	if n.IsInt {
		return strconv.FormatInt(n.Int, 10)
	}
	return strconv.FormatFloat(n.Float, 'f', -1, 64)
}

// decimals counts digits after the decimal point.
func (n number) decimals() (int, bool) {
	s := n.text()
	i := strings.LastIndex(s, ".")
	if i < 0 {
		return 0, false
	}
	return len(s[i+1:]), true
}

// digits counts significant digits, ignoring sign, point and exponent, which
// is what the size: rule means for a number.
func (n number) digits() int {
	s := n.text()
	count := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			count++
		}
		if r == 'e' || r == 'E' {
			break
		}
	}
	return count
}

func (n number) isNaN() bool { return math.IsNaN(n.Float) }
