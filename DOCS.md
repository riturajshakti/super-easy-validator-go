# super-easy-validator-go — Full Documentation

- [Guide](#guide) — how each feature works, with runnable examples
- [Reference](#reference) — every rule, one entry each, and the full code vocabulary

```sh
go get github.com/riturajshakti/super-easy-validator-go
```

```go
import validator "github.com/riturajshakti/super-easy-validator-go"
```

---

# Guide

1. [The call and the result](#1-the-call-and-the-result)
2. [Separators](#2-separators)
3. [Data](#3-data)
4. [Automatic type checks](#4-automatic-type-checks)
5. [Numeric and boolean strings](#5-numeric-and-boolean-strings)
6. [Numbers, without bit widths](#6-numbers-without-bit-widths)
7. [Argument based validations](#7-argument-based-validations)
8. [Regular expressions](#8-regular-expressions)
9. [Nested object validation](#9-nested-object-validation)
10. [Simple array validation](#10-simple-array-validation)
11. [Array objects validation](#11-array-objects-validation)
12. [Array indexing](#12-array-indexing)
13. [`$atleast` and `$atmost`](#13-atleast-and-atmost)
14. [Combining rules with `$or` and `$and`](#14-combining-rules-with-or-and-and)
15. [Conditional rules with `$switch`](#15-conditional-rules-with-switch)
16. [Custom rules](#16-custom-rules)
17. [Error options](#17-error-options)
18. [Strict check](#18-strict-check)
19. [Error ordering](#19-error-ordering)

## 1. The call and the result

`Validate` takes a `Rules` map and a `Data` map, and returns a `Result` and an
`error`:

```go
result, err := validator.Validate(rules, data)
result, err := validator.Validate(rules, data, validator.Config{Strict: true})
```

The two return values mean different things, and the distinction matters:

- **`err`** is an `*InvalidRuleError`: your **rules** are malformed — a typo, a
  bad operator node. This is a programmer error, not a validation failure, and
  no amount of different input would fix it. `result` is empty when it is set.
- **`result`** carries the validation outcome for the **data**.

```go
result, err := validator.Validate(rules, data)
if err != nil {
    log.Fatal(err) // fix your rules
}
if !result.Valid() {
    for _, message := range result.Errors {
        fmt.Println(message)
    }
}
```

`MustValidate` is the same call for code that treats a malformed rule as fatal
— it panics instead of returning an error, which suits package-level rule
variables:

```go
var userRules = validator.Rules{"name": "fullname", "age": "natural|min:18"}

func handler(data validator.Data) {
    result := validator.MustValidate(userRules, data)
    // ...
}
```

### The result

```go
type Result struct {
    Errors  []string
    Details []Detail
}

type Detail struct {
    Field   string
    Message string
    Code    ErrorCode
}
```

`Errors` is `nil` when everything passed, so `result.Valid()` and
`result.Errors != nil` say the same thing. When validation fails, `Errors` and
`Details` are the same length and share indexes:
`Errors[i] == Details[i].Message`.

`Detail.Field` carries the path, so you need not parse it back out of the
message — which is what you want when returning errors from an HTTP handler:

```go
result, _ := validator.Validate(
    validator.Rules{"age": "natural|min:18"},
    validator.Data{"age": 15},
)

result.Errors
// ["age must be at least 18"]

result.Details[0]
// {Field: "age", Message: "age must be at least 18", Code: "TOO_SMALL"}
```

Turning failures into a JSON response:

```go
type fieldError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

out := make([]fieldError, len(result.Details))
for i, d := range result.Details {
    out[i] = fieldError{d.Field, d.Message, d.Code}
}
json.NewEncoder(w).Encode(map[string]any{"errors": out})
```

## 2. Separators

`Validate` requires a `Rules` map whose keys are the names of the fields to
validate. The values are the rules, as a string, separated by the pipe `|`
operator without any spaces. You can give multiple rules for any value:

```go
rules := validator.Rules{
    "name": "string|min:2|max:8",
}
```

Three rules are applied to `name`:

1. `string` — the field must be a string
2. `min:2` — it must be at least 2 characters long
3. `max:8` — it must be at most 8 characters long

> **Note:** instead of a pipe-separated string you can give the rules as a
> `[]string`. Here is the equivalent of the rule above:

```go
rules := validator.Rules{
    "name": []string{"string", "min:2", "max:8"},
}
```

The list form exists for rules that contain a literal `|` — a regex
alternation or an error message. See [§8](#8-regular-expressions) and
[§17](#17-error-options).

> Note also that the list form means something different for `$atleast` and
> `$atmost`, where it applies several independent groups. See
> [`$atleast`](#atleast) in the Reference.

## 3. Data

`Data` is `map[string]any` — the shape `encoding/json` produces:

```go
var data validator.Data
json.Unmarshal(body, &data)

result, err := validator.Validate(rules, data)
```

### The three states

| State | Written as | Satisfied by |
|---|---|---|
| absent | key not in the map | `optional` |
| null | key present, value `nil` | `nullable` |
| present | key present, value set | every other rule |

These are three genuinely distinct states:

```go
validator.Validate(validator.Rules{"f": "optional|string"}, validator.Data{})
// passes

validator.Validate(validator.Rules{"f": "optional|string"}, validator.Data{"f": nil})
// "f is required"

validator.Validate(validator.Rules{"f": "nullable|string"}, validator.Data{"f": nil})
// passes

validator.Validate(validator.Rules{"f": "nullable|string"}, validator.Data{})
// "f is required"
```

Use both when either is acceptable: `"optional|nullable|string"`.

> **Note:** a JSON `null` decodes to `nil`, so `{"f": null}` is the *nullable*
> case. A client omitting the key entirely is the *optional* case. If your API
> should accept both spellings, use `optional|nullable`.

### Why not structs

A struct cannot express the table above. `encoding/json` decodes both an
explicit `null` and a missing key to the same nil pointer, so `optional` and
`nullable` would become indistinguishable:

```
map:    a present=true value=<nil>   (null)
        c present=false              (absent)

struct: A=<nil> C=<nil>              (null and missing are the same)
```

Supporting structs would mean a second input path with weaker semantics, plus
a reflection layer to drive it. Marshal a struct to a map first:

```go
b, _ := json.Marshal(myStruct)
var data validator.Data
json.Unmarshal(b, &data)
```

### Presence is not truthiness

A present-but-falsy value is a *type* error, not a missing one:

```go
rules := validator.Rules{"users": []validator.Rules{{"name": "string"}}}

// {"users": ""}    -> "users must be of type array"
// {"users": 0}     -> "users must be of type array"
// {"users": false} -> "users must be of type array"
// {"users": nil}   -> "users is required"
```

## 4. Automatic type checks

String rules imply `string`; number rules imply `number`. You do not write
`"string|email"` — `"email"` already rejects a non-string.

These rules check for the `string` data type automatically:

`email` · `url` · `domain` · `name` · `fullname` · `username` · `alpha` ·
`alphanumeric` · `phone` · `phonecode` · `objectid` · `uuid` · `date` ·
`dateonly` · `time` · `lower` · `upper` · `ip` · `regex:<value>`

```go
validator.Rules{"a": "email"}   // a non-string fails with "a must be string"
```

And these check for `number`:

`int` · `positive` · `negative` · `natural` · `whole`

```go
validator.Rules{"b": "natural"} // a non-number fails with "b must be a valid number"
```

## 5. Numeric and boolean strings

Prefix with `string` to validate a value that arrives as text — form fields,
query parameters, CSV columns:

```go
validator.Rules{"age":    "string|natural"}  // "25" passes, 25 fails
validator.Rules{"active": "string|boolean"}  // "true" passes, true fails
```

The message changes to match:

```
age must be a valid natural numeric string
active must be a valid boolean string
```

The distinction is driven by a `string` token *earlier in the same rule*.
`"string|number"` accepts `"42"`; plain `"number"` rejects it.

## 6. Numbers, without bit widths

Go has eleven numeric types; a rule string has one idea of "number". Integer
rules (`int`, `natural`, `whole`) work in integers, and `number` and the
decimal rules work in floats. You never write `int32` or `float64` in a rule,
and every Go numeric kind is accepted:

```go
for _, v := range []any{int(5), int8(5), int64(5), uint(5), float32(5), float64(5)} {
    validator.Validate(validator.Rules{"f": "natural"}, validator.Data{"f": v})
    // all pass
}
```

Internally the widest types are used — `int64` and `float64` — so range is
never the binding constraint, and a narrow value is widened once at the
boundary rather than carrying its precision artifacts into digit counting.

Integer-ness is decided by **value**, not static type, so a JSON `7` — which
decodes to `float64` — satisfies `natural`:

```go
// {"f": 7}   -> natural passes
// {"f": 7.5} -> "f must be a valid natural number"
```

`NaN` and `Infinity` are valid for the plain `number` rule, since both are
`float64` values, but neither satisfies `int`, `natural` or `whole` — neither
is an integer. `NaN` also fails the decimal rules, with `NOT_A_NUMBER`.

For integers beyond `float64` precision, see [`bigint`](#bigint).

## 7. Argument based validations

These rules take an argument after a colon:

- `equal:<value>` — equality check for string, number and boolean
- `size:<int>` — length check for strings and slices, digit-count for numbers
- `min:<value>` — minimum length, minimum value, or not-before for dates
- `max:<value>` — maximum length, maximum value, or not-after for dates
- `regex:<regex>` — regular expression check for strings
- `decimalsize:<value>` — exact number of digits after the decimal point
- `decimalmin:<value>` — minimum number of digits after the decimal point
- `decimalmax:<value>` — maximum number of digits after the decimal point
- `enums:<value>` — comma-separated allowed values
- `arrayof:<validation>` — applies the given validation to every element

```go
rules := validator.Rules{
    "code":  "string|size:6",
    "age":   "natural|min:18|max:120",
    "price": "number|decimalmax:2",
    "role":  "enums:admin,user,guest",
}
```

Several are overloaded by the value's runtime type — see [`size:`](#size),
[`min:`](#min) and [`max:`](#max) in the Reference.

## 8. Regular expressions

A pattern may be written in JavaScript literal form, and is translated to Go's
inline-flag form automatically:

```go
validator.Rules{"hash": `regex:/^[A-Z0-9]{128}$/i`}   // becomes (?i)^[A-Z0-9]{128}$
validator.Rules{"hash": `regex:(?i)^[A-Z0-9]{128}$`}  // already in Go form
```

Flags `i`, `m` and `s` map to `(?i)`, `(?m)`, `(?s)`. `g`, `y` and `u` are
accepted and ignored: `g` and `y` are stateful-iteration flags meaningless for
this check, and Go patterns are UTF-8 aware by default.

Use a **raw string literal** (backticks) so backslashes reach the regex engine
intact.

### What RE2 cannot do

Go's `regexp` implements RE2, which guarantees linear-time matching by not
implementing backtracking at all. **Lookahead, lookbehind and backreferences
are absent from the engine**, not merely spelled differently, so no amount of
rewriting produces an equivalent:

```go
validator.Rules{"f": `regex:/^(?!www)x$/`}
// error: 'f' uses negative lookahead '(?!', which Go's regexp engine does not support

validator.Rules{"f": `regex:/^(a)\1$/`}
// error: 'f' uses a backreference, which Go's regexp engine does not support
```

Such a pattern is reported as an `*InvalidRuleError` naming the field and the
unsupported construct, rather than silently never matching.

In practice most validation patterns — character classes, anchors,
quantifiers, alternation, Unicode property escapes — are unaffected. This
restriction applies **only** to caller-supplied patterns: every built-in format
rule works as documented.

### A pattern containing a pipe

The `|` inside a pattern would be read as a rule separator, so use the list
form:

```go
// wrong — the | splits the rule
validator.Rules{"gender": `string|regex:/^(male)|(female)$/`}

// right
validator.Rules{"gender": []string{"string", `regex:/^(male)|(female)$/`}}
```

## 9. Nested object validation

Nest `Rules` as deep as you like:

```go
rules := validator.Rules{
    "address": validator.Rules{
        "line1": "string|min:10",
        "line2": "optional|string|min:10",
        "city":  "name",
        "state": "name",
        "pin":   "string|natural|size:6",
        "country": validator.Rules{
            "code":      "alpha|upper|size:2",
            "phoneCode": `regex:/^\+[0-9]{1,3}$/`,
        },
    },
}

data := validator.Data{
    "address": validator.Data{
        "line1": 800,
        "line2": false,
        "city":  "N/A",
        "state": "N/A",
        "pin":   "ABC123",
        "country": validator.Data{
            "phoneCode": "+9999",
        },
    },
}

result, _ := validator.Validate(rules, data)
for _, message := range result.Errors {
    fmt.Println(message)
}
```

### Output

```
address.line1 must be string
address.line2 must be string
address.city must be a valid name
address.state must be a valid name
address.pin must be a valid numeric string
address.country.code is required
address.country.phoneCode is invalid
```

There is no limit on the depth of nesting.

A missing or `nil` nested object reports `is required`; a non-map reports
`must be of type object`.

### A dotted key asserts its intermediates are objects

You can reach a nested field with a dotted key, without nesting the rules:

```go
validator.Rules{"person.address.city": "name"}
```

A key of `a.b.c` says three things: `a` is an object, `a.b` is an object, and
`a.b.c` satisfies the rule. When an intermediate holds something else, the
segment that is wrong is reported — not the leaf it hid:

```go
// {"a": 5}          -> "a must be of type object"
// {"a": {"b": 5}}   -> "a.b must be of type object"
// {"a": {"b": {}}}  -> "a.b.c is required"
```

The code is `NOT_OBJECT`, the same one the equivalent nested rule produces, so
`{"a": {"b": "string"}}` and `{"a.b": "string"}` report identically for the
same bad data.

`optional` does not suppress this. It excuses a field that is **absent**; an
`a` that is present and is not an object is a type error. A nil intermediate
counts as absent, so `nullable` data still reads as missing:

```go
rules := validator.Rules{"a.b.c": "optional|string"}
// {}                -> passes  (absent)
// {"a": nil}        -> passes  (null reads as absent)
// {"a": 5}          -> "a must be of type object"
// {"a": {"b": {}}}  -> passes  (leaf absent, and optional)
```

A custom rule is the exception: it owns its own decision about absence, so a
broken chain reaches it as a `nil` value rather than as an error.

## 10. Simple array validation

To validate every element of a slice, use `arrayof:`. It is an argument-based
rule whose argument is itself a rule:

```go
rules := validator.Rules{
    "skills": "array|min:2|arrayof:string|arrayof:max:10",
}
```

This passes only when all of the following hold:

- `skills` is a slice
- it has at least 2 elements
- every element is a string
- no element exceeds 10 characters

Numeric strings work the same way:

```go
rules := validator.Rules{
    "codes": "arrayof:string|arrayof:int",
}
```

And a slice whose elements may be `nil`:

```go
rules := validator.Rules{
    "emails": "arrayof:optional|arrayof:email",
}
```

> **Note:** the `array` check is applied automatically when you use `arrayof:`.

Native Go slices work as data, not just `[]any`:

```go
validator.Data{"tags": []string{"a", "b"}}   // accepted by array and arrayof:
```

## 11. Array objects validation

To validate objects inside a slice, give a **one-element `[]Rules`**:

```go
rules := validator.Rules{
    "products": []validator.Rules{{
        "title":       "string|min:5",
        "description": "string|min:20",
        "price":       "positive",
        "category":    "enums:Electronics,Kitchen,Fashion,Others",
    }},
}

data := validator.Data{
    "products": []any{
        validator.Data{
            "title":       "Smartphone",
            "description": "High-performance smartphone with a stunning display.",
            "price":       599.99,
        },
        validator.Data{
            "title":    "Coffee Maker",
            "price":    "InvalidPrice",
            "category": "Kitchen",
        },
        validator.Data{
            "title":       "Designer Dress",
            "description": "Elegant and stylish designer dress for special occasions.",
            "price":       149.99,
            "category":    "InvalidCategory",
        },
    },
}

result, _ := validator.Validate(rules, data)
```

### Output

```
products[0].category is required
products[1].description is required
products[1].price must be a valid number
products[2].category is invalid
```

Tuple rules nest, for a slice of slices of objects:

```go
validator.Rules{"grid": []any{[]validator.Rules{{"label": "string"}}}}
// [[{"label": 1}]] -> "grid[0][0].label must be string"
// ["x"]            -> "grid[0] must be of type array"
// [["x"]]          -> "grid[0][0] must be of type object/array"
```

There is no limit on nesting depth, and you can freely combine nested object
rules with tuple rules.

## 12. Array indexing

Rule keys can select individual slice elements, count from the end, or select
a range:

```go
rules := validator.Rules{
    "coords":      "array|size:2",
    "coords[0]":   "number|min:-90|max:90",   // latitude
    "coords[1]":   "number|min:-180|max:180", // longitude
    "history[-1]": "date",                    // the most recent entry
}

validator.Validate(rules, validator.Data{"coords": []any{200, -0.12}})
// "coords[0] must be at most 90"
```

### The three forms

| Form | Meaning |
|---|---|
| `c[0]` | the element at index 0 |
| `c[-1]` | the last element; `c[-2]` the second to last |
| `c[0:2]` | elements 0 and 1 — the rule applies to **each** selected element |

A bracket selects elements, so the rule you write is an *element* rule. Use a
plain key when you want to validate the slice itself:

```go
validator.Rules{"c": "array|min:3|arrayof:natural"} // the slice, and every element
validator.Rules{"c[0]": "natural"}                  // just the first element
validator.Rules{"c[0:2]": "natural"}                // each of the first two
```

Negative indexes count from the end, and the label echoes what you wrote
(`history[-1]`, not the resolved position). Slice labels *are* resolved.

### Slices

Bounds follow Go's own slicing: the start is inclusive, the end exclusive,
either may be omitted, and both may be negative. Out-of-range bounds clamp
rather than error.

```go
validator.Rules{"c[1:]": "number"}    // from index 1 to the end
validator.Rules{"c[:2]": "number"}    // the first two
validator.Rules{"c[-2:]": "number"}   // the last two
validator.Rules{"c[5:9]": "number"}   // selects nothing on a short slice, so it passes
```

Errors are reported per element, with the real index:

```go
validator.Validate(
    validator.Rules{"c[0:2]": "number"},
    validator.Data{"c": []any{"a", "b", 3}},
)
// ["c[0] must be a valid number", "c[1] must be a valid number"]
```

### Combining with paths

Indexing composes with dotted paths, in either direction, and with nested
slices:

```go
validator.Rules{"a.c[0]": "number"}        // index inside a nested object
validator.Rules{"u[0].name": "string"}     // a property of an indexed element
validator.Rules{"u[0:2].name": "string"}   // that property on each selected element
validator.Rules{"order.items[0].sku": "string|min:3"}
validator.Rules{"c[0][1]": "number"}       // nested slices
```

### Missing elements

An index that does not exist reads as a missing field, so it reports
`is required` and can be made optional:

```go
validator.Rules{"c[9]": "number"}           // [1] -> "c[9] is required"
validator.Rules{"c[9]": "optional|number"}  // [1] -> passes
```

The same applies when the field is absent or is not a slice.

### Turning indexing off

Set `Config{DisableArrayIndexing: true}` to treat bracketed keys literally.
Use this if your data genuinely contains keys such as `"c[0]"`:

```go
validator.Validate(
    validator.Rules{"c[0]": "string"},
    validator.Data{"c[0]": "x"},
    validator.Config{DisableArrayIndexing: true},
)
// passes: the key is matched literally
```

This affects only bracket syntax. Dotted paths and ordinary keys behave the
same either way.

### Strict mode

An indexed rule declares its base key, so `"c[0]"` counts `c` as declared:

```go
// {"c": []any{1}}          with Strict -> no errors
// {"c": []any{1}, "z": 2}  with Strict -> "z is not required"
```

## 13. `$atleast` and `$atmost`

Sometimes at least one field is required from a set of otherwise optional
fields:

```go
rules := validator.Rules{
    "mail":     "optional|email",
    "phone":    "optional|phone",
    "$atleast": "mail|phone",
}
// {} -> "at least one of mail and phone is required"
```

To require at least `n` of a set, use the `size:` keyword. The `size:` part is
not counted as a field:

```go
validator.Rules{"$atleast": "a|b|c|size:2"}
// {"a": "x"} -> "at least 2 of a, b and c are required"
```

`$atmost` caps instead:

```go
validator.Rules{"$atmost": "mail|phone"}
// both given -> "at most one of mail and phone can be given"
```

Pass a `[]string` to apply several independent groups, each producing its own
error:

```go
validator.Rules{"$atleast": []string{"a|b", "x|y"}}
```

Both count **presence, not truthiness** — a field set to `false` or `0` still
counts as given.

## 14. Combining rules with `$or` and `$and`

Operators are **map keys**. A rule value whose single key is `$or`, `$and` or
`$switch` is an operator node:

```go
validator.Rules{"id": validator.Rules{"$or": []any{"objectid", "uuid"}}}
```

An operator node carries that one key and nothing else. Mixing it with a
sibling key is a rule error, so wrap them in a branch instead.

Because rules are plain maps, an operator tree can be decoded from JSON or
assembled at runtime:

```go
var rules validator.Rules
json.Unmarshal([]byte(`{
    "address": {"$or": ["string|max:60", {"city":"name","pin":"string|natural|size:6"}]}
}`), &rules)
```

A typed builder is available if you prefer it, and produces the identical node:

```go
validator.Rules{"id": validator.Operator{Or: []any{"objectid", "uuid"}}}
```

### Semantics

- **`$or`** passes if **any** branch passes. If every branch fails, it reports
  the errors of the branch that best fits the value.
- **`$and`** passes only if **every** branch passes. It reports the failures of
  all branches, so you see every unmet requirement at once.

### What a branch can be

A branch is anything a rule value can be, so operators compose with everything
else in the library:

| Branch | Example |
|---|---|
| a rule string | `"string\|max:20"` |
| a list of rule tokens | `[]string{"string", "regex:..."}` |
| an object rule | `validator.Rules{"city": "name"}` |
| a tuple rule | `[]validator.Rules{{"title": "string"}}` |
| a custom function | `validator.CustomRule(isEven)` |
| another operator | `validator.Rules{"$or": ...}` |

### Making a nested object optional

A plain object rule is always required:

```go
validator.Validate(validator.Rules{"addr": validator.Rules{"city": "name"}}, validator.Data{})
// "addr is required"
```

Use `$and` with `optional` to allow it to be absent:

```go
rules := validator.Rules{
    "addr": validator.Rules{"$and": []any{
        "optional",
        validator.Rules{"city": "name", "pin": "string|natural|size:6"},
    }},
}

// {}                                            -> no errors
// {"addr": {"city": "London", "pin": "123456"}} -> no errors
// {"addr": {"city": "123", "pin": "123456"}}    -> "addr.city must be a valid name"
```

`nullable` works the same way for `nil`, and both can be combined:

```go
validator.Rules{"profile": validator.Rules{"$and": []any{
    "optional|nullable",
    validator.Rules{"bio": "string|max:200"},
}}}
```

### Making a slice of objects optional

The same pattern applies to tuple rules:

```go
rules := validator.Rules{
    "products": validator.Rules{"$and": []any{
        "optional",
        []validator.Rules{{"title": "string|min:5", "price": "positive"}},
    }},
}

// {}                                   -> no errors
// {"products": []any{}}                -> no errors
// {"products": [{"title": "ab", ...}]} -> "products[0].title must have length of at least 5"
```

### Branch selection when every `$or` branch fails

Reporting every branch's errors would be noisy — a three-branch `$or` could
emit a dozen messages for one bad value. Instead `$or` reports a single
branch's errors, chosen by:

1. **Type match first.** Branches whose expected type matches the value's
   runtime type are preferred. Map data is checked against object branches,
   slice data against array branches, and so on.
2. **Fewest errors next.** Among those, the branch with the fewest failures
   wins.
3. **Declaration order last.** Ties go to the branch written first.

This is why the errors are specific rather than vague:

```go
rules := validator.Rules{"address": validator.Rules{"$or": []any{
    "string|max:20",
    validator.Rules{"city": "name"},
}}}

// {"address": {"city": "123"}}
//   -> "address.city must be a valid name"     (not "address is invalid")

// {"address": "a very long street name indeed"}
//   -> "address must have length of at most 20"
```

### Discriminated unions

Because a branch can be a whole object rule, tagged unions work directly:

```go
rules := validator.Rules{
    "payment": validator.Rules{"$or": []any{
        validator.Rules{"type": "equal:card", "number": `string|regex:/^[0-9]{16}$/`},
        validator.Rules{"type": "equal:upi", "upiId": "string|min:5"},
        validator.Rules{"type": "equal:bank", "account": "string|natural|min:9"},
    }},
}
```

### Reporting every failure with `$and`

A rule string stops at the first failure. `$and` reports all of them, which is
what a password-strength UI needs:

```go
rules := validator.Rules{
    "password": validator.Rules{"$and": []any{
        "string|min:12",
        []string{`regex:[A-Z]`, "error:must contain an uppercase letter"},
        []string{`regex:[0-9]`, "error:must contain a digit"},
    }},
}

// {"password": "abc"} ->
//   "password must have length of at least 12"
//   "must contain an uppercase letter"
//   "must contain a digit"
```

### `$and` merges its object branches

Under `Strict`, a key declared in **any** `$and` branch counts as declared,
since all branches must pass:

```go
rules := validator.Rules{"v": validator.Rules{"$and": []any{
    validator.Rules{"a": "string"},
    validator.Rules{"b": "string"},
}}}

// {"v": {"a": "x", "b": "y"}}           with Strict -> no errors
// {"v": {"a": "x", "b": "y", "c": 1}}   with Strict -> "v.c is not required"
```

### Nesting

Operators nest to any depth, in any combination, and inside nested objects and
tuple rules:

```go
// $or inside $and
validator.Rules{"a": validator.Rules{"$and": []any{
    "optional", validator.Rules{"$or": []any{"string|max:5", validator.Rules{"c": "name"}}},
}}}

// $and inside $or
validator.Rules{"a": validator.Rules{"$or": []any{
    validator.Rules{"$and": []any{"string", "min:3"}}, "number",
}}}

// operator inside a nested object
validator.Rules{"user": validator.Rules{"contact": validator.Rules{"$or": []any{"email", "phone"}}}}

// operator inside a tuple rule
validator.Rules{"contacts": []validator.Rules{{"value": validator.Rules{"$or": []any{"email", "phone"}}}}}
```

Error paths are preserved throughout — `contacts[1].value`, `a.b.c`, and so on.

### Rules for writing an operator node

An operator node must contain **only** its operator key. These return an
`*InvalidRuleError`:

```go
validator.Rules{"a": validator.Rules{"$or": []any{"string"}, "city": "name"}}     // mixed with another key
validator.Rules{"a": validator.Rules{"$and": []any{"string"}, "$or": []any{"x"}}} // two operators
validator.Rules{"a": validator.Rules{"$or": "string"}}                            // branches must be a list
validator.Rules{"a": validator.Rules{"$or": []any{}}}                             // at least one branch
```

A `$or` branch cannot consist only of `optional` and/or `nullable`. Such a
branch accepts *any* present value, so it would silently swallow every failure
from the other branches:

```go
// rejected — the invalid name would otherwise be accepted
validator.Rules{"address": validator.Rules{"$or": []any{"optional", validator.Rules{"city": "name"}}}}

// correct — $and makes the field optional and still validates it when present
validator.Rules{"address": validator.Rules{"$and": []any{"optional", validator.Rules{"city": "name"}}}}
```

Combining a modifier with a real rule is fine, because the branch still
constrains the value:

```go
validator.Rules{"address": validator.Rules{"$or": []any{"optional|string", validator.Rules{"city": "name"}}}}
```

### How operators interact with plain object rules

A `Rules` value is treated as an operator node **only** when it contains `$or`,
`$and` or `$switch`. Any other map is a normal nested-object rule and keeps its
usual behaviour, including being required by default.

### Strict mode with `$or`

With `Strict` and a `$or` of object branches, the keys of the branch that
matched count as declared:

```go
// rules: {"a": {"$or": [{"c": "name"}]}}
// data:  {"a": {"c": "John", "extra": 1}}   with Strict -> "a.extra is not required"
```

When no branch matches, strict errors from the rejected branches are
suppressed, so you see why the value failed rather than noise about keys that
belonged to a branch it was never going to match.

## 15. Conditional rules with `$switch`

`$switch` picks **one** rule to apply, based on which case the value matches.
Where `$or` asks "does this match *any* of these?", `$switch` asks "which rule
*applies* here?" — it commits to a branch, then validates against only that
branch, so the errors are specific rather than vague.

```go
validator.Rules{"amount": validator.Rules{"$switch": []any{
    map[string]any{"case": "number|max:1000", "then": "positive", "default": true},
    map[string]any{"case": "number|min:1001", "then": "positive|decimalmax:2"},
}}}
```

| Value | Outcome |
|---|---|
| `500` | first case matches → `"positive"` applies |
| `5000` | second case matches → `"positive\|decimalmax:2"` applies |
| `5000.123` | second case matches → `must have at most 2 decimal places` |
| `"abc"` | no case matches → the `default` branch's `then` applies → `must be a valid number` |

### How a branch is chosen

1. Each branch's **`case`** is tested against the value, in order.
2. The **first** case that passes wins, and its **`then`** is applied. No later
   case is evaluated.
3. If **no** case passes, the branch marked **`"default": true`** supplies the
   rule, so the error reads as that branch would report it.
4. If no case passes and there is no default, the field reports
   `does not match any case` with the code `NO_CASE_MATCHED`.

`"default": true` does not disable that branch's `case` — the branch still
matches normally. It only says "use this branch's `then` when nothing matched".

### Branch shape

Each branch is a `map[string]any` with `case`, `then`, and optionally
`default`. Both `case` and `then` accept **any** rule value:

```go
validator.Rules{"a": validator.Rules{"$switch": []any{
    map[string]any{"case": "object", "then": validator.Rules{"city": "name"}},
    map[string]any{"case": "array", "then": []validator.Rules{{"title": "string"}}},
    map[string]any{"case": "string", "then": validator.Rules{"$and": []any{"min:3", "max:20"}}},
}}}
```

The typed builder produces the same node:

```go
validator.Rules{"amount": validator.Operator{Switch: []validator.SwitchBranch{
    {Case: "number|max:1000", Then: "positive", Default: true},
    {Case: "number|min:1001", Then: "positive|decimalmax:2"},
}}}
```

### Switching on another field

A `case` may be a custom function, which receives the parent map — so
switching on a sibling field needs no special syntax:

```go
isImage := validator.CustomRule(func(value, parent any) *validator.RuleError {
    p, _ := parent.(validator.Data)
    if p["type"] == "image" {
        return nil
    }
    return &validator.RuleError{Message: "skip", Code: "SKIP"}
})

rules := validator.Rules{
    "type": "enums:image,video",
    "meta": validator.Rules{"$switch": []any{
        map[string]any{"case": isImage, "then": validator.Rules{"width": "natural", "height": "natural"}},
    }},
}
```

A bad image payload reports `meta.height is required`, not a vague "meta is
invalid" — which is the main reason to reach for `$switch` over `$or`.

### Combining with `$and` and `$or`

`$switch` composes with the other operators in both directions:

```go
// a switch inside $and
validator.Rules{"a": validator.Rules{"$and": []any{
    "number", validator.Rules{"$switch": []any{map[string]any{"case": "max:10", "then": "positive"}}},
}}}

// operators inside a then
validator.Rules{"a": validator.Rules{"$switch": []any{
    map[string]any{"case": "number", "then": validator.Rules{"$or": []any{"min:100", "max:1"}}},
}}}

// optional, via $and
validator.Rules{"a": validator.Rules{"$and": []any{
    "optional", validator.Rules{"$switch": []any{map[string]any{"case": "number", "then": "positive"}}},
}}}
```

It works anywhere a rule value is accepted: nested objects, tuple rules, dotted
paths and indexed keys.

### Rules for writing a `$switch`

These return an `*InvalidRuleError`:

```go
validator.Rules{"a": validator.Rules{"$switch": "x"}}             // must be a list
validator.Rules{"a": validator.Rules{"$switch": []any{}}}         // needs a branch
validator.Rules{"a": validator.Rules{"$switch": []any{"number"}}} // a branch must be a map
```

A branch also needs a `case` and a `then`; unknown branch keys are rejected;
`default` must be a boolean; only **one** branch may be the default; and
`$switch` must be the only key in its node.

### Strict mode

When a `$switch` selects an object `then`, that branch's keys count as
declared:

```go
// rules: {"a": {"$switch": [{"case": "object", "then": {"c": "name"}}]}}
// data:  {"a": {"c": "John", "z": 1}}   with Strict -> "a.z is not required"
```

## 16. Custom rules

A rule value can be a function:

```go
type CustomRule func(value, parent any) *RuleError
```

Return `nil` to pass, or a `*RuleError` to fail. You own the message and the
code:

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

`parent` is the enclosing object at that level, which is what makes cross-field
checks work without special syntax. Inside a nested object the parent is that
object; inside a tuple rule it is the element.

Common uses:

```go
// a value that depends on another field
taxID := validator.CustomRule(func(value, parent any) *validator.RuleError {
    p, _ := parent.(validator.Data)
    if p["type"] == "business" && value == nil {
        return &validator.RuleError{
            Message: "taxId is required for business accounts",
            Code:    "REQUIRED_FOR_BUSINESS",
        }
    }
    return nil
})

// a total that must match its parts
total := validator.CustomRule(func(value, parent any) *validator.RuleError {
    p, _ := parent.(validator.Data)
    items, _ := p["items"].([]any)
    sum := 0.0
    for _, it := range items {
        if n, ok := it.(float64); ok {
            sum += n
        }
    }
    if v, ok := value.(float64); ok && v == sum {
        return nil
    }
    return &validator.RuleError{Message: "total does not match items", Code: "BAD_TOTAL"}
})
```

Two things to know:

- **The function runs even when the value is absent**, receiving `nil`. It owns
  the decision about absence — there is no implicit `is required`. An absent
  key and an explicit `null` both arrive as `nil`.
- **The code is yours.** It is not restricted to the built-in vocabulary.
  Omit it and it defaults to `CUSTOM_RULE_FAILED`.

### Combining with built-in rules

A function is a complete rule value, so combine it with string rules using
`$and`:

```go
validator.Rules{"n": validator.Rules{"$and": []any{"natural", validator.CustomRule(isEven)}}}
```

A function also works as a `$or` branch, where it can satisfy the operator on
its own.

### Where a custom rule may appear

Anywhere a rule value is accepted: a top-level field, inside a nested object
rule, inside a tuple rule (running once per element), on a dotted path, on an
indexed or negative-indexed key, on a slice key (running once per selected
element), and as a branch of `$or`, `$and`, or as a `$switch` `case` or `then`.

```go
validator.Rules{"a.n": validator.CustomRule(isEven)}
validator.Rules{"c[0]": validator.CustomRule(isEven)}
validator.Rules{"c[0:2]": validator.CustomRule(isEven)}
validator.Rules{"u": []validator.Rules{{"n": validator.CustomRule(isEven)}}}
```

### The return contract

The signature keeps most mistakes out of reach: a custom rule returns
`*RuleError` or nothing, so a wrong return type is a compile error rather than
a runtime surprise. Two runtime checks remain:

- Returning a `RuleError` with an **empty `Message`** is a rule error naming
  the field.
- A **panic** inside a custom rule is recovered and reported as a rule error
  naming the field, rather than crashing the caller.

A plain func literal works without the `CustomRule` conversion in most
positions:

```go
validator.Rules{"n": func(v, _ any) *validator.RuleError { return nil }}
```

## 17. Error options

### `field:` — rename the field

```go
validator.Rules{"age": "natural|field:person age"}
// -> "person age must be a valid natural number"
```

Suppressed for keys ending in an array index, where the index is the meaningful
label.

### `error:` — replace the message

```go
validator.Rules{"age": "natural|error:given age is not valid"}
// -> "given age is not valid"
```

The **code is preserved** — only the text changes, so machine consumers still
see `NOT_NATURAL`.

Both must come last in the rule. A message containing `|` needs the list form:

```go
validator.Rules{"name": []string{"name", "error:bad | worse"}}
```

### `Quotes` — wrap field names

```go
validator.Validate(rules, data, validator.Config{Quotes: validator.QuoteDouble})
```

| Setting | Output |
|---|---|
| `QuoteNone` (default) | `name must be a valid name` |
| `QuoteSingle` | `'name' must be a valid name` |
| `QuoteDouble` | `"name" must be a valid name` |
| `QuoteTick` | `` `name` must be a valid name `` |

## 18. Strict check

Sometimes the data must contain *only* the fields named in the rules:

```go
rules := validator.Rules{
    "name":   "name",
    "age":    "natural|min:18",
    "gender": "enums:male,female",
}

user := validator.Data{
    "name":   "john doe",
    "age":    30,
    "gender": "male",
    "hobby":  "web development",
}

result, _ := validator.Validate(rules, user, validator.Config{Strict: true})
```

### Output

```
hobby is not required
```

Strict mode applies to nested objects and tuple rules too (`a.c is not
required`, `u[0].b is not required`), and is off by default.

## 19. Error ordering

Go randomizes map iteration, so **error order within one rules level is
unspecified**. Ordering between nesting levels is stable, since that follows
recursion rather than map iteration.

Sort when you need deterministic output:

```go
sort.Slice(result.Details, func(i, j int) bool {
    return result.Details[i].Field < result.Details[j].Field
})
```

One field reports at most one error; `$and` and `arrayof:` may report several.
Identical messages collapse, keeping the first occurrence's code — two fields
sharing a custom `error:` text produce one entry.

---

# Reference

Every rule, one entry each. Rules marked **auto** imply a type check, so you
need not write `string` or `number` first.

## Presence

### `optional`

Passes when the key is **absent** from the map. Does not accept `nil`.

```go
validator.Rules{"organization": "optional|string"}
```

In the example above, `organization` may be missing, or it must be a string.

### `nullable`

Passes when the key is present and holds `nil`. Does not accept absence.

```go
validator.Rules{"organization": "nullable|string"}
```

> **Note:** to allow both absence and `nil`, apply both:

```go
validator.Rules{"age": "optional|nullable|number"}
```

### `$atleast`

At least N of the named fields must be present. `size:` sets N, defaulting to
1, and is not counted as a field. Counts **presence, not truthiness** — a field
set to `false` still counts as given. Code: `ATLEAST_NOT_MET`.

```go
validator.Rules{
    "otpUnlock":  "optional|boolean",
    "faceUnlock": "optional|boolean",
    "pinUnlock":  "optional|boolean",
    "$atleast":   "otpUnlock|faceUnlock|pinUnlock",
}
```

With a threshold:

```go
validator.Rules{"$atleast": "name|email|username|phone|size:2"}
```

To check several groups independently, pass a `[]string`. Each group produces
its own error:

```go
validator.Rules{"$atleast": []string{"name|age|gender|size:2", "id|email|username"}}
```

### `$atmost`

At most N of the named fields may be present. Code: `ATMOST_EXCEEDED`.

```go
validator.Rules{"$atmost": "otpUnlock|faceUnlock|pinUnlock"}
validator.Rules{"$atmost": "name|email|username|phone|size:2"}
validator.Rules{"$atmost": []string{"name|age|gender|size:2", "id|email|username"}}
```

## Data types

### `string`

Checks that the field is a string. Code: `NOT_STRING` → `f must be string`.

```go
validator.Rules{"fieldName": "string"}
```

> **Note:** this check is applied automatically by `email`, `url`, `domain`,
> `name`, `fullname`, `username`, `alpha`, `alphanumeric`, `phone`,
> `phonecode`, `objectid`, `uuid`, `date`, `dateonly`, `time`, `lower`,
> `upper`, `ip` and `regex:<value>`.

### `number`

Checks that the field is a number. Accepts every Go numeric kind — `int`,
`int8`…`int64`, `uint`…`uint64`, `float32`, `float64`. Code: `NOT_NUMBER`.

```go
validator.Rules{"a": "number"}        // 42 passes, "42" fails
validator.Rules{"b": "string|number"} // "42" passes, 42 fails
```

With a preceding `string`, this validates a numeric string instead, reporting
`NOT_NUMERIC_STRING`.

> **Note:** this check is applied automatically by `int`, `positive`,
> `negative`, `natural` and `whole`.

### `boolean`

Checks that the field is `true` or `false`. Code: `NOT_BOOLEAN`.

```go
validator.Rules{"fieldName": "boolean"}
```

> **Note:** combine `string` and `boolean` to check for a boolean string, which
> accepts only `"true"` and `"false"` (`NOT_BOOLEAN_STRING`):

```go
validator.Rules{"active": "string|boolean"}
```

### `array`

Checks that the field is a slice. Accepts `[]any` and native Go slices such as
`[]string` or `[]int`. Code: `NOT_ARRAY`.

```go
validator.Rules{"fieldName": "array"}
```

> **Note:** a string is not an array, even though it is indexable.

### `object`

Checks that the field is a map. Accepts `Data` and `map[string]any`. Code:
`NOT_OBJECT`.

```go
validator.Rules{
    "address":      "object",
    "address.pin":  `regex:^[0-9]{6}$`,
    "address.city": "name",
}
```

You can also write rules for nested properties using a dotted key, as above.

> **Note:** a slice fails this check. Slices and maps have different roles
> here, so neither satisfies the other.

### `bigint`

Checks for an integer beyond `float64` precision. Accepts `*big.Int` and
`json.Number`. Code: `NOT_BIGINT`.

Plain `encoding/json` decoding into `any` destroys precision past 2^53,
silently:

```go
// {"big": 12345678901234567890} -> 1.2345678901234567e+19
```

Decoding with `UseNumber()` preserves the literal, so the value survives a JSON
round trip exactly:

```go
dec := json.NewDecoder(r)
dec.UseNumber()
dec.Decode(&data)

validator.Rules{"big": "bigint"}   // 12345678901234567890 stays exact
```

```go
validator.Rules{"f": "bigint"}     // big.NewInt(42) passes; 42 fails
```

## String formats

> **Note:** these rules check the `string` data type automatically.

### `email`

Checks for a valid email string. Code: `NOT_EMAIL` →
`f must be a valid email`.

```go
validator.Rules{"myEmail": "email"}
```

Accepts `a@b.com`, `first.last@sub.domain.co.uk` and `o'brien@mail.org`.
Rejects `plainstring`, `a@`, `@b.com` and `a@b`.

### `url`

Checks for a valid URL string. Code: `NOT_URL`.

```go
validator.Rules{"profile": "url"}
```

Accepts `https://example.com`, `http://www.example.com/path` and
`www.example.com`. Rejects a bare `example.com` and `ftp://x`.

### `domain`

Checks for a valid domain string. Code: `NOT_DOMAIN`.

```go
validator.Rules{"site": "domain"}
```

Accepts `example.com` and `sub-domain.co.in`. Rejects `https://example.com`.

### `name`

Checks for a valid name — a full name or a short one. Code: `NOT_NAME`.

```go
validator.Rules{"studentName": "name"}
```

Letters from any script are accepted, so `José`, `李小龙`, `राज` and
`Владимир` are all valid. A name may contain spaces, apostrophes, hyphens and a
trailing period on any word — `Jean-Luc`, `O'Brien` and `Dr. Smith` all pass.
Digits, underscores and repeated or leading separators are rejected.

### `fullname`

Like `name`, but requires **at least two** space-separated parts. Code:
`NOT_FULLNAME`.

```go
validator.Rules{"studentName": "fullname"}
```

`María García`, `山田 太郎` and `राज कुमार` are all valid. `John` alone is
not.

### `username`

Checks for a valid username. Code: `NOT_USERNAME`.

```go
validator.Rules{"username": "username"}
```

8–20 characters, alphanumeric plus `.` and `_`, with no leading or trailing
separator and no two consecutive separators. `johndoe123` and `john.doe.x`
pass; `john..doe`, `_johndoe` and `ab` fail.

### `alpha`

Checks that a string contains only letters, upper or lower case. Code:
`NOT_ALPHA`. Pattern: `[A-Za-z]+`.

```go
validator.Rules{"keyword": "alpha"}
```

### `alphanumeric`

Checks that a string contains only letters and digits. Code:
`NOT_ALPHANUMERIC`. Pattern: `[A-Za-z0-9]+`.

```go
validator.Rules{"passwordHash": "alphanumeric"}
```

### `phone`

Checks for a valid phone number: an optional `+` country code, an optional
parenthesised group, digits and spaces. Code: `NOT_PHONE`.

```go
validator.Rules{"phone": "optional|phone"}
```

Accepts `+91 9876543210`, `9876543210`, `+1 (555)1234567` and `555 123 4567`.

### `phonecode`

Checks for a valid country calling code — `+` followed by 1–3 digits. Code:
`NOT_PHONECODE`.

```go
validator.Rules{"phoneCode": "phonecode"}
```

Accepts `+1`, `+91`, `+963`. Rejects `91`, `+1234` and `+`.

### `objectid`

Checks for a valid MongoDB ObjectId — 24 hexadecimal characters. Code:
`NOT_OBJECTID`.

```go
validator.Rules{"userId": "objectid"}
```

> **Note:** the JavaScript package also accepts `mongoid` as a deprecated
> alias. This package does not — use `objectid`.

### `uuid`

Checks for a valid UUID, as used for SQL table identifiers. Code: `NOT_UUID`.

```go
validator.Rules{"userId": "uuid"}
```

Accepts `123e4567-e89b-12d3-a456-426655440000`, in either case.

### `date`

Checks for a valid ISO 8601 date, optionally including a time and zone. Code:
`NOT_DATE`.

```go
validator.Rules{"dob": "date"}
```

This accepts all of:

```
1996-01-10
1996-01-10T23:50:34
1996-01-10T23:50:34.6789Z
1996-01-10T23:50:34.6789+05:30
```

### `dateonly`

Checks for a valid ISO date with **no** time component — strictly
`YYYY-MM-DD`. Code: `NOT_DATEONLY`.

```go
validator.Rules{"dob": "dateonly"}
```

> `date` and `dateonly` produce the same message but different codes.

### `time`

Checks for a valid 24-hour time with no date component. Code: `NOT_TIME`.

```go
validator.Rules{"startedAt": "time"}
```

Accepts `23:55`, `23:55:00` and `23:55:00.34`. Rejects `25:00`.

### `lower`

Checks that a string contains no uppercase letters. Digits, spaces and
punctuation are all allowed. Code: `NOT_LOWERCASE` →
`f must not contains upper case letters`.

```go
validator.Rules{"slug": "lower"}
```

### `upper`

Checks that a string contains no lowercase letters. Code: `NOT_UPPERCASE`.

```go
validator.Rules{"code": "upper"}
```

### `ip`

Checks for a valid IPv4 address. Code: `NOT_IP`.

```go
validator.Rules{"host": "ip"}
```

Accepts `8.45.23.0`, `255.255.255.255` and `0.0.0.0`. Rejects `256.1.1.1`,
`1.2.3` and `localhost`.

## Number types

> **Note:** these rules check the `number` data type automatically. Each has a
> numeric-string variant when preceded by `string`, with the **same code** and
> a `numeric string` message.

### `int`

Checks that a number has no fractional part. Code: `NOT_INTEGER`.

```go
validator.Rules{"temperature": "int"}
validator.Rules{"temperature": "string|int"}   // an integer string
```

A float with no fractional part counts as an integer, so `5.0` passes.

### `positive`

Checks that a number is `> 0`. Code: `NOT_POSITIVE`.

```go
validator.Rules{"price": "positive"}
validator.Rules{"price": "string|positive"}
```

### `negative`

Checks that a number is `< 0`. Code: `NOT_NEGATIVE`.

```go
validator.Rules{"depth": "negative"}
validator.Rules{"depth": "string|negative"}
```

### `natural`

Checks for a positive integer. Code: `NOT_NATURAL`.

```go
validator.Rules{"iq": "natural"}
validator.Rules{"iq": "string|natural"}
```

### `whole`

Checks for a positive integer or `0`. Code: `NOT_WHOLE`.

```go
validator.Rules{"score": "whole"}
validator.Rules{"score": "string|whole"}
```

> **Note:** `NaN` and `Infinity` are valid `number`s but satisfy none of these
> five rules, because neither is an integer value.

## Constraints

### `equal:`

Checks that a field has a specific value. Works for string, number and boolean,
detecting the type from the value. Code: `NOT_EQUAL`.

```go
validator.Rules{
    "marks":    "number|equal:100",       // a number equal to 100
    "isTopper": "boolean|equal:true",     // a boolean equal to true
    "name":     "string|equal:sia",       // a string equal to "sia"
    "status":   "equal:200",              // auto-detect
}
```

For `status`, if the value is a string it must be `"200"`; if it is a number it
must be `200`. A value that is not a string, number or boolean is not compared.

### `size:`

Overloaded by runtime type:

| Value | Meaning | Code |
|---|---|---|
| string | rune count | `LENGTH_MISMATCH` |
| array | element count | `LENGTH_MISMATCH` |
| number | digit count, ignoring sign and point | `DIGITS_MISMATCH` |

```go
validator.Rules{
    "foods":   "array|size:3",          // exactly 3 elements
    "otp":     "string|natural|size:6", // a 6-digit natural number string
    "pinCode": "number|size:5",         // a 5-digit number
    "field":   "size:4",                // auto-detect
}
```

> **Note:** when no data type is given, as with `field` above, the type is
> detected and the check applied only if the value is a string, number or
> slice.

> **Note:** for numbers, only the digits are counted — a minus sign, decimal
> point and exponent are ignored. So `-123`, `1.23` and `123` all have size 3.

> **Note:** string length counts **runes**, so `"日本"` has length 2 and an
> emoji has length 1. Languages that count UTF-16 code units disagree on the
> latter.

### `min:`

Overloaded four ways:

| Value | Meaning | Code |
|---|---|---|
| string | minimum rune count | `TOO_SHORT` |
| array | minimum element count | `TOO_SHORT` |
| number | minimum value | `TOO_SMALL` |
| date (non-numeric argument) | not before | `DATE_TOO_EARLY` |

```go
validator.Rules{
    "foods":   "array|min:3",
    "otp":     "string|natural|min:6",
    "pinCode": "number|min:5",
    "dob":     "date|min:2024-01-01",
    "field":   "min:4",                 // auto-detect
}

// {"dob": "2023-06-01"} -> "dob must be at least 2024-01-01T00:00:00.000Z"
```

### `max:`

The mirror of `min:` — `TOO_LONG`, `TOO_LARGE` and `DATE_TOO_LATE`.

```go
validator.Rules{
    "foods":   "array|max:3",
    "pinCode": "number|max:5",
    "dob":     "date|max:2024-12-31",
}
```

### `regex:`

Checks that a string matches a regular expression. Implies `string`. Written in
literal (`/pattern/flags`) or inline-flag form. Code: `REGEX_MISMATCH` →
`f is invalid`; a non-string reports `NOT_STRING`.

```go
validator.Rules{"hash": `regex:/^[A-Z0-9]{128}$/i`}
```

> **Important:** Go's regexp is RE2, so lookahead, lookbehind and
> backreferences are unavailable and are reported as rule errors. See
> [Guide §8](#8-regular-expressions).

> **Note:** a pattern containing `|` needs the list form, or the `|` is read as
> a rule separator.

### `decimalsize:`

Checks for exactly N digits after the decimal point. Code:
`DECIMAL_SIZE_MISMATCH`.

```go
validator.Rules{
    "price": "string|number|decimalsize:2",
    "pi":    "number|decimalsize:6",
    "rate":  "decimalsize:3",   // auto-detect
}
```

> **Warning:** for a `number`, trailing zeroes are lost before the check,
> because binary floating point cannot represent them. `23.340` is
> indistinguishable from `23.34`, so `decimalsize:3` fails for it. Numeric
> strings keep exactly what you wrote:

```go
validator.Rules{"a": "string|number|decimalsize:2"} // "10.50" passes
validator.Rules{"b": "number|decimalsize:3"}        // 23.340 fails
```

### `decimalmin:`

At least N digits after the decimal point. Code: `DECIMAL_TOO_FEW`. The same
trailing-zero caveat applies.

### `decimalmax:`

At most N digits after the decimal point. Code: `DECIMAL_TOO_MANY`. The same
caveat applies, so `decimalmax:0` accepts any whole-valued float.

### `enums:`

Checks that a field holds one of a comma-separated set. Branches on the value's
runtime type — strings compare literally, numbers numerically, booleans against
`true`/`false`. Code: `ENUM_MISMATCH` → `f is invalid`.

```go
validator.Rules{
    "rating":    "number|enums:1,2,3,4,5",
    "status":    "string|enums:pending,success,failed",
    "isMarried": "boolean|enums:true,false",
    "subject":   "enums:english,maths,science",   // auto-detect
}
```

> **Warning:** do not add spaces between the values — they become part of the
> value. Write `"enums:A+,A,B+,B,C,F"`, not `"enums: A+, A, B+"`.

### `field:`

Renames the field in messages. Must come last. Suppressed for keys ending in an
array index.

```go
validator.Rules{"age": "natural|field:person age"}
```

### `error:`

Replaces the message, preserving the code. Must come last.

```go
validator.Rules{"age": "natural|error:given age is not valid"}
```

## Arrays

### `arrayof:<rule>`

Applies any rule above to every element, labelling failures by index. Nests
arbitrarily. A non-slice fails with `NOT_ARRAY`.

```go
validator.Rules{"skills": "array|min:2|arrayof:string|arrayof:max:10"}
// ["ok", "waaaaaaaaaytoolong"] -> "skills[1] must have length of at most 10"
```

`arrayof:optional` and `arrayof:nullable` allow `nil` elements:

```go
validator.Rules{"emails": "arrayof:optional|arrayof:email"}
// ["a@b.com", nil] passes
```

It composes with itself to any depth:

```go
validator.Rules{"matrix": "arrayof:arrayof:number"}
// [[1], ["x"]] -> "matrix[1][0] must be a valid number"

validator.Rules{"cube": "arrayof:arrayof:arrayof:number"}
// -> "cube[0][0][0] must be a valid number"
```

#### Supported inner rules

**Optional and nullable elements:** `arrayof:optional`, `arrayof:nullable`

**Data types:** `arrayof:string`, `arrayof:number`, `arrayof:boolean`,
`arrayof:array`, `arrayof:object`, `arrayof:bigint`

**String formats:** `arrayof:email`, `arrayof:url`, `arrayof:domain`,
`arrayof:name`, `arrayof:fullname`, `arrayof:username`, `arrayof:alpha`,
`arrayof:alphanumeric`, `arrayof:phone`, `arrayof:phonecode`,
`arrayof:objectid`, `arrayof:uuid`, `arrayof:date`, `arrayof:dateonly`,
`arrayof:time`, `arrayof:lower`, `arrayof:upper`, `arrayof:ip`

**Number types:** `arrayof:int`, `arrayof:positive`, `arrayof:negative`,
`arrayof:natural`, `arrayof:whole`

**Argument based:** `arrayof:equal:`, `arrayof:size:`, `arrayof:min:`,
`arrayof:max:`, `arrayof:regex:`, `arrayof:decimalsize:`,
`arrayof:decimalmin:`, `arrayof:decimalmax:`, `arrayof:enums:`

> Each follows the same behaviour described in its own section, applied to
> every element.

### Error paths

Slice indexes always use bracket notation and map keys use dots, at any
combination of depth:

| Structure | Example path |
|---|---|
| `arrayof:` | `tags[0]` |
| nested `arrayof:` | `matrix[1][0]` |
| `arrayof:` inside an object | `user.tags[0]` |
| tuple rule | `products[2].title` |
| nested tuple rule | `grid[0][1].label` |
| object inside a tuple rule | `orders[0].address.city` |

## Operators

### `$or`

Passes if any branch passes. On failure, reports the best-fitting branch:
type-matching branches first, then fewest errors, then declaration order. A
branch that is only `optional`/`nullable` is a rule error.

See [Guide §14](#14-combining-rules-with-or-and-and) for branch selection,
discriminated unions and the authoring rules.

### `$and`

Every branch must pass, and every branch's failures are reported. Use with
`optional` to make a nested object or tuple rule optional. Plain object
branches are merged, so `Strict` sees their union.

### `$switch`

One rule chosen by the first passing `case`. `"default": true` marks the
fallback; without one, a non-matching value reports `NO_CASE_MATCHED`.

See [Guide §15](#15-conditional-rules-with-switch) for branch shape, switching
on a sibling field, and composition.

## Error codes

`Validate` returns `Details` alongside `Errors`. Both are the same length and
share indexes, so `Details[i].Message == Errors[i]`.

```go
result, _ := validator.Validate(
    validator.Rules{"age": "natural|min:18"},
    validator.Data{"age": 15},
)

result.Errors     // ["age must be at least 18"]
result.Details[0] // {Field: "age", Message: "age must be at least 18", Code: "TOO_SMALL"}
```

Both are `nil` when validation passes.

### Why codes

Message text is for humans and may change between releases. Codes are stable
and are meant to be compared against:

```go
for _, d := range result.Details {
    if d.Code == validator.CodeRequired {
        // a field was missing
    }
}
```

They also separate failures that share a message. `enums:` and `regex:` both
produce `is invalid`:

```go
result, _ := validator.Validate(
    validator.Rules{"e": "enums:a,b", "r": `regex:^a$`},
    validator.Data{"e": "z", "r": "z"},
)
// e -> ENUM_MISMATCH, r -> REGEX_MISMATCH, both reading "is invalid"
```

A custom `error:` replaces the message but keeps the code, so you can show your
own wording and still branch on the cause.

### All codes

```go
validator.CodeRequired // "REQUIRED"
```

| Group | Codes |
|---|---|
| Presence | `REQUIRED` `DATA_REQUIRED` `DATA_NOT_OBJECT` `UNEXPECTED_FIELD` |
| Types | `NOT_STRING` `NOT_NUMBER` `NOT_NUMERIC_STRING` `NOT_BOOLEAN` `NOT_BOOLEAN_STRING` `NOT_ARRAY` `NOT_OBJECT` `NOT_BIGINT` |
| String formats | `NOT_EMAIL` `NOT_URL` `NOT_DOMAIN` `NOT_NAME` `NOT_FULLNAME` `NOT_USERNAME` `NOT_ALPHA` `NOT_ALPHANUMERIC` `NOT_PHONE` `NOT_PHONECODE` `NOT_OBJECTID` `NOT_UUID` `NOT_DATE` `NOT_DATEONLY` `NOT_TIME` `NOT_IP` |
| Case | `NOT_LOWERCASE` `NOT_UPPERCASE` |
| Number types | `NOT_INTEGER` `NOT_POSITIVE` `NOT_NEGATIVE` `NOT_NATURAL` `NOT_WHOLE` |
| Size and bounds | `LENGTH_MISMATCH` `DIGITS_MISMATCH` `TOO_SHORT` `TOO_LONG` `TOO_SMALL` `TOO_LARGE` `DATE_TOO_EARLY` `DATE_TOO_LATE` `NOT_EQUAL` |
| Decimals | `DECIMAL_SIZE_MISMATCH` `DECIMAL_TOO_FEW` `DECIMAL_TOO_MANY` `NOT_A_NUMBER` |
| Matching | `ENUM_MISMATCH` `REGEX_MISMATCH` |
| Groups | `ATLEAST_NOT_MET` `ATMOST_EXCEEDED` |
| Operators | `NO_BRANCH_MATCHED` `NO_CASE_MATCHED` `CUSTOM_RULE_FAILED` |
| Internal | `INVALID_RULE` `INTERNAL_ERROR` |

> The numeric-string forms (`string|natural` and so on) use the same codes as
> their plain counterparts — the message says which form it was.

> `min:`/`max:` mean different things by type, so they get different codes. A
> 2-character password is `TOO_SHORT`; an age of 17 is `TOO_SMALL`.

A custom rule's code is not restricted to this list.
