# super-easy-validator-go

**Validate data with rules you write as plain strings.** Zero dependencies, fully typed. No builder chains, no struct tags — just `"optional|email"`.

The Go port of the npm package [super-easy-validator](https://github.com/riturajshakti/super-easy-validator).

```sh
go get github.com/riturajshakti/super-easy-validator-go
```

**[📖 Full documentation — guide and complete API reference](DOCS.md)**

## Why

```go
// super-easy-validator-go
validator.Rules{"age": "optional|natural|min:18"}

// the usual alternative
type User struct {
    Age *int `validate:"omitempty,gt=0,min=18"`
}
```

- **Zero runtime dependencies**
- Rules are data, so they can be built at runtime, loaded from config, or shared
- Validates decoded JSON directly — no struct required
- Nested objects, arrays of objects, per-element array rules, custom messages

## Quick start

```go
package main

import (
    "fmt"

    validator "github.com/riturajshakti/super-easy-validator-go"
)

func main() {
    rules := validator.Rules{
        "name":     "fullname",
        "email":    "email",
        "password": "string|min:8",
        "age":      "optional|natural|min:18",
        "role":     "enums:admin,user,guest",
        "website":  "optional|url",
    }

    data := validator.Data{
        "name":     "John",
        "email":    "not-an-email",
        "password": "abc",
        "age":      15,
        "role":     "superuser",
        "website":  "example.com",
    }

    result, err := validator.Validate(rules, data)
    if err != nil {
        panic(err) // a malformed rule, not a validation failure
    }
    for _, e := range result.Errors {
        fmt.Println(e)
    }
}
```

```
name must be a valid fullname
email must be a valid email
password must have length of at least 8
age must be at least 18
role is invalid
website must be a valid url
```

`result.Valid()` reports success. Alongside `Errors`, the `Details` slice pairs each message with a stable code and the field it came from:

```go
result, _ := validator.Validate(validator.Rules{"age": "natural|min:18"}, validator.Data{"age": 15})
// result.Details[0] -> {Field: "age", Message: "age must be at least 18", Code: "TOO_SMALL"}
```

## Data, and the three states

`Data` is `map[string]any` — the direct analogue of a JSON object:

| State | Written as | Satisfies |
|---|---|---|
| absent | key not in the map | `optional` |
| null | key present, value `nil` | `nullable` |
| present | key present, value set | the rest of the rules |

Structs are not accepted: `encoding/json` decodes both `null` and a missing key to the same nil pointer, so the distinction would be lost. Marshal a struct to a map first.

## Structure: nested objects, arrays, and indexing

```go
rules := validator.Rules{
    "address": validator.Rules{
        "city":    "name",
        "pin":     "string|natural|size:6",
        "country": validator.Rules{"code": "alpha|upper|size:2"},
    },
    "tags":  "array|min:2|arrayof:string|arrayof:max:10",
    "users": []validator.Rules{{"name": "name", "age": "natural"}},
    "grid":  []any{[]validator.Rules{{"label": "string"}}}, // arrays of arrays of objects

    "coords":      "array|size:2",
    "coords[0]":   "number|min:-90|max:90",   // latitude
    "coords[1]":   "number|min:-180|max:180", // longitude
    "history[-1]": "date",                    // the most recent entry
    "matrix":      "arrayof:arrayof:number",  // arrays of arrays
}
```

Errors carry the full path, including array indexes:

```
address.pin must be a valid numeric string
address.country.code must not contains lower case letters
tags[1] must have length of at most 10
users[1].name is required
coords[0] must be at most 90
history[-1] must be a valid date
matrix[1][0] must be a valid number
```

Slices work too — `"c[0:2]"`, `"c[1:]"`, `"c[-2:]"` — applying the rule to each selected element.

## Operators: `$or`, `$and`, `$switch`

`$or` passes if any branch passes. `$and` requires every branch. `$switch` applies one rule, chosen by which `case` matches. Branches accept any rule value: strings, nested rules, tuple rules, functions, or nested operators.

Operators are map keys, exactly as in the npm package:

```go
rules := validator.Rules{
    // one field, several valid shapes
    "address": validator.Rules{"$or": []any{
        "string|max:60",
        validator.Rules{"city": "name", "pin": "string|natural|size:6"},
    }},

    // $and with optional makes a nested object or array optional
    "billing": validator.Rules{"$and": []any{
        "optional",
        validator.Rules{"line1": "string|min:5", "city": "name"},
    }},

    // one rule chosen by case; default supplies the error when nothing matches
    "amount": validator.Rules{"$switch": []any{
        map[string]any{"case": "number|max:1000", "then": "positive", "default": true},
        map[string]any{"case": "number|min:1001", "then": "positive|decimalmax:2"},
    }},
}
```

`$or` reports the branch that best fits the value, so you get `address.pin ...` rather than a vague "address is invalid". `$and` merges its object branches, so a key declared in any branch counts as declared under `Strict`.

Because operators are plain map keys, a whole rule tree can be decoded from JSON or built at runtime. A typed `Operator` builder is also available when you prefer it:

```go
validator.Rules{"id": validator.Operator{Or: []any{"objectid", "uuid"}}}
```

## Custom rules and cross-field validation

A rule value can be a function. It receives the value and its parent, and returns `nil` to pass or a `*RuleError` to fail — so you own the wording and the code, and cross-field checks need no special syntax.

```go
rules := validator.Rules{
    "password": "string|min:8",
    "confirmPassword": validator.CustomRule(func(value, parent any) *validator.RuleError {
        p, _ := parent.(validator.Data)
        if value == p["password"] {
            return nil
        }
        return &validator.RuleError{Message: "passwords must match", Code: "PASSWORD_MISMATCH"}
    }),
}
```

Combine a function with built-in rules through `$and`:

```go
validator.Rules{"n": validator.Rules{"$and": []any{"natural", validator.CustomRule(isEven)}}}
```

A panic inside a custom rule is recovered and reported as a rule error, naming the field.

The function is called even when the value is absent, so it owns the decision about absence.

## Every rule at a glance

Combine rules with `|`, or pass a `[]string` — `[]string{"string", "min:3"}` — when a rule contains a `|` itself.

| Group | Rules |
|---|---|
| **Presence** | `optional` `nullable` `$atleast` `$atmost` |
| **Types** | `string` `number` `boolean` `array` `object` `bigint` |
| **Strings** | `email` `url` `domain` `name` `fullname` `username` `alpha` `alphanumeric` `phone` `phonecode` `objectid` `uuid` `date` `dateonly` `time` `lower` `upper` `ip` |
| **Numbers** | `int` `positive` `negative` `natural` `whole` |
| **Constraints** | `equal:` `size:` `min:` `max:` `regex:` `decimalsize:` `decimalmin:` `decimalmax:` `enums:` |
| **Arrays** | `arrayof:<any rule above>` |
| **Operators** | `$or` `$and` `$switch` |
| **Messages** | `field:` `error:` and a `Quotes` option |

String rules check for a string automatically; number rules check for a number. Prefix with `string` to validate numeric or boolean strings — `"string|natural"`, `"string|boolean"`.

An unknown rule is returned as an `*InvalidRuleError` rather than being ignored, so a typo surfaces at first run.

## Numbers

Callers never reason about bit widths. Integer rules (`int`, `natural`, `whole`) work in integers; `number` and the decimal rules work in floats. Every Go numeric kind normalizes to that pair, so `int`, `int8`…`int64`, `uint`…`uint64`, `float32` and `float64` all behave identically.

Integer-ness is decided by value, not static type, so a JSON `7` (decoded as `float64`) satisfies `natural`.

`bigint` accepts `*big.Int` and `json.Number`, so an integer too large for `float64` survives a JSON round trip — something the npm package cannot do.

## Regular expressions

Patterns are written in JavaScript literal form and translated automatically:

```go
"regex:/^[A-Z0-9]{128}$/i"   // becomes (?i)^[A-Z0-9]{128}$
```

Go's `regexp` is RE2, which has no backtracking. Lookahead, lookbehind and backreferences are reported as rule errors rather than silently never matching. See [LIMITATIONS.md](LIMITATIONS.md).

## Options

```go
validator.Validate(rules, data, validator.Config{
    Quotes: validator.QuoteTick,
    Strict: true,
})
```

- **`Quotes`** — `QuoteNone` (default), `QuoteSingle`, `QuoteDouble`, `QuoteTick`
- **`Strict`** — reject any field in the data that has no rule, nested objects included
- **`DisableArrayIndexing`** — treat keys like `"c[0]"` as literal names instead of array indexes

## Error ordering

Go randomizes map iteration, so **error order within one rules level is unspecified**. Ordering between nesting levels is stable. Sort by `Detail.Field` when you need deterministic output.

## Differences from the npm package

See [LIMITATIONS.md](LIMITATIONS.md) for the full list. In short: `symbol` is not supported, `regex:` cannot use backtracking constructs, error order is unspecified, and input is `map[string]any` rather than also accepting structs.

## License

MIT
