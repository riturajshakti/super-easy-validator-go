# super-easy-validator-go — Full Documentation

The Go port of the npm package [super-easy-validator](https://github.com/riturajshakti/super-easy-validator).

- [Guide](#guide) — how each feature works, with runnable examples
- [Reference](#reference) — every rule, one entry each
- [Error codes](#error-codes) — the full code vocabulary
- [Differences from npm](#differences-from-the-npm-package)

```sh
go get github.com/riturajshakti/super-easy-validator-go
```

```go
import validator "github.com/riturajshakti/super-easy-validator-go"
```

---

# Guide

## 1. The call

```go
result, err := validator.Validate(rules, data)
result, err := validator.Validate(rules, data, validator.Config{Strict: true})
```

`Validate` returns two things, and they mean different things:

- **`err`** is an `*InvalidRuleError`: your rules are malformed (a typo, a bad operator). This is a programmer error, not a validation failure. `result` is empty when it is set.
- **`result`** carries the validation outcome.

```go
result, err := validator.Validate(rules, data)
if err != nil {
    log.Fatal(err) // fix your rules
}
if !result.Valid() {
    for _, e := range result.Errors {
        fmt.Println(e)
    }
}
```

`MustValidate` is the same call for code that treats a malformed rule as fatal — it panics instead of returning `err`.

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

`Errors` is `nil` when everything passed. `Errors` and `Details` are the same length and index-aligned: `Errors[i] == Details[i].Message`.

`Detail.Field` is the Go port's addition — npm has no equivalent. It carries the field path without parsing it back out of the message, which is what you want for a JSON API response:

```go
result, _ := validator.Validate(validator.Rules{"age": "natural|min:18"}, validator.Data{"age": 15})
// Details[0] = {Field: "age", Message: "age must be at least 18", Code: "TOO_SMALL"}
```

## 2. Separators

Combine rules with `|`:

```go
validator.Rules{"age": "optional|natural|min:18"}
```

Use a `[]string` when a rule contains a literal `|` — a regex alternation or an error message:

```go
validator.Rules{"gender": []string{"string", `regex:/^(male)|(female)$/`}}
validator.Rules{"name": []string{"name", "error:bad | worse"}}
```

Both forms are equivalent otherwise.

## 3. Data

`Data` is `map[string]any` — the direct analogue of a JSON object. It is what `encoding/json` produces:

```go
var data validator.Data
json.Unmarshal(body, &data)
result, _ := validator.Validate(rules, data)
```

### The three states

| State | Written as | Satisfied by |
|---|---|---|
| absent | key not in the map | `optional` |
| null | key present, value `nil` | `nullable` |
| present | key present, value set | every other rule |

These are three distinct states, and the rules distinguish them:

```go
ok    := validator.Rules{"f": "optional|string"} // {} passes
notOK := validator.Rules{"f": "optional|string"} // {"f": nil} fails: "f is required"
ok2   := validator.Rules{"f": "nullable|string"} // {"f": nil} passes
notOK2:= validator.Rules{"f": "nullable|string"} // {} fails: "f is required"
```

Use both when either is acceptable: `"optional|nullable|string"`.

### Why not structs

A struct cannot express the table above. `encoding/json` decodes both an explicit `null` and a missing key to the same nil pointer, so `optional` and `nullable` would become indistinguishable. Marshal a struct to a map first:

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

String rules imply `string`; number rules imply `number`. You do not write `"string|email"` — `"email"` already rejects a non-string:

```go
validator.Rules{"a": "email"}    // a non-string fails with "a must be string"
validator.Rules{"b": "natural"}  // a non-number fails with "b must be a valid number"
```

## 5. Numeric and boolean strings

Prefix with `string` to validate a value that arrives as text — form fields, query parameters, CSV columns:

```go
validator.Rules{"age":    "string|natural"}  // "25" passes, 25 fails
validator.Rules{"active": "string|boolean"}  // "true" passes, true fails
```

The message changes to match:

```
age must be a valid natural numeric string
active must be a valid boolean string
```

The distinction is driven by a `string` token *earlier in the same rule*. `"string|number"` accepts `"42"`; plain `"number"` rejects it.

## 6. Numbers, without bit widths

Integer rules (`int`, `natural`, `whole`) work in integers; `number` and the decimal rules work in floats. You never write `int32` or `float64` in a rule, and every Go numeric kind is accepted:

```go
for _, v := range []any{int(5), int8(5), int64(5), uint(5), float32(5), float64(5)} {
    validator.Validate(validator.Rules{"f": "natural"}, validator.Data{"f": v}) // all pass
}
```

Integer-ness is decided by **value**, not static type, so a JSON `7` — which decodes to `float64` — satisfies `natural`:

```go
// {"f": 7}   -> natural passes
// {"f": 7.5} -> "f must be a valid natural number"
```

## 7. Argument rules

Nine rules take an argument after a colon:

```go
validator.Rules{
    "code":  "string|size:6",
    "age":   "natural|min:18|max:120",
    "price": "number|decimalmax:2",
    "role":  "enums:admin,user,guest",
}
```

Several are overloaded by the value's runtime type — see [`size`](#size), [`min`](#min) and [`max`](#max) in the reference.

## 8. Regular expressions

Patterns are written in JavaScript literal form and translated for you:

```go
validator.Rules{"hash": `regex:/^[A-Z0-9]{128}$/i`}  // becomes (?i)^[A-Z0-9]{128}$
```

Flags `i`, `m`, `s` map to Go's inline form. `g`, `y` and `u` are dropped — they mean nothing for a full-string test. A pattern already in Go form works too:

```go
validator.Rules{"hash": `regex:(?i)^[A-Z0-9]{128}$`}
```

A pattern containing `|` needs the list form, or the `|` is read as a rule separator:

```go
// wrong
validator.Rules{"gender": `string|regex:/^(male)|(female)$/`}
// right
validator.Rules{"gender": []string{"string", `regex:/^(male)|(female)$/`}}
```

**Lookahead, lookbehind and backreferences are not available.** Go's regexp is RE2, which has no backtracking. Such a pattern is reported as an `*InvalidRuleError` rather than silently never matching:

```go
_, err := validator.Validate(validator.Rules{"f": `regex:/^(?!www)x$/`}, data)
// err: 'f' uses negative lookahead '(?!', which Go's regexp engine does not support
```

## 9. Nested objects

Nest `Rules` as deep as you like:

```go
rules := validator.Rules{
    "address": validator.Rules{
        "city":    "name",
        "country": validator.Rules{"code": "alpha|upper|size:2"},
    },
}
```

Errors carry the dotted path:

```
address.country.code must not contains lower case letters
address.country.code is required
```

A missing or null nested object reports `is required`; a non-object reports `must be of type object`.

Dot-notation keys work too, and are useful for reaching one deep field without nesting:

```go
validator.Rules{"person.address.city": "name"}
```

### A dotted key asserts its intermediates are objects

`"a.b.c"` says three things: `a` is an object, `a.b` is an object, and `a.b.c`
satisfies the rule. When an intermediate holds something else, the segment that
is wrong is reported — not the leaf it hid:

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

## 10. Arrays

Three different things, often confused:

```go
validator.Rules{
    "tags":  "array|min:2",              // the array itself
    "names": "arrayof:string",           // every element
    "users": []validator.Rules{{ ... }}, // every element, as an object
}
```

### `arrayof:` — every element

`arrayof:` takes any rule and applies it per element, labelling failures by index:

```go
validator.Rules{"tags": "array|min:2|arrayof:string|arrayof:max:10"}
// ["ok", "waaaaaaaaaytoolong"] -> "tags[1] must have length of at most 10"
```

It nests arbitrarily:

```go
validator.Rules{"matrix": "arrayof:arrayof:number"}
// [[1], ["x"]] -> "matrix[1][0] must be a valid number"
```

`arrayof:optional` and `arrayof:nullable` let elements be absent:

```go
validator.Rules{"mails": "arrayof:optional|arrayof:email"}
// ["a@b.com", nil] passes
```

### Tuple rules — arrays of objects

A one-element `[]Rules` validates every element as an object:

```go
rules := validator.Rules{"users": []validator.Rules{{"name": "name", "age": "natural"}}}
// [{"name":"Jo","age":20}, {}] -> "users[1].name is required"
//                                 "users[1].age is required"
```

Tuple rules nest, for arrays of arrays of objects:

```go
validator.Rules{"grid": []any{[]validator.Rules{{"label": "string"}}}}
// [[{"label": 1}]] -> "grid[0][0].label must be string"
// ["x"]            -> "grid[0] must be of type array"
// [["x"]]          -> "grid[0][0] must be of type object/array"
```

Native Go slices work as data, not just `[]any`:

```go
validator.Data{"tags": []string{"a", "b"}} // accepted by array and arrayof:
```

## 11. Array indexing

Target specific elements from the rule key:

```go
rules := validator.Rules{
    "coords":      "array|size:2",
    "coords[0]":   "number|min:-90|max:90",   // latitude
    "coords[1]":   "number|min:-180|max:180", // longitude
    "history[-1]": "date",                    // the most recent entry
}
```

Negative indexes count from the end, and the label echoes what you wrote (`history[-1]`, not the resolved position).

Slices apply the rule to each selected element, and their labels *are* resolved:

```go
validator.Rules{"c[0:2]": "number"}
// ["a", "b", 3] -> "c[0] must be a valid number"
//                  "c[1] must be a valid number"
```

Open ends and negatives work: `"c[1:]"`, `"c[:2]"`, `"c[-2:]"`. Indexes chain, and mix with dots: `"order.items[0].sku"`, `"m[0][1]"`.

An out-of-range index reads as missing, so `optional` covers it:

```go
validator.Rules{"c[9]": "number"}           // [1] -> "c[9] is required"
validator.Rules{"c[9]": "optional|number"}  // [1] -> passes
```

Set `Config{DisableArrayIndexing: true}` to treat `"c[0]"` as a literal key name.

## 12. `$atleast` and `$atmost`

Require or cap how many of a set of fields are present:

```go
rules := validator.Rules{
    "mail":     "optional|email",
    "phone":    "optional|phone",
    "$atleast": "mail|phone",
}
// {} -> "at least one of mail and phone is required"
```

Add `size:` for a threshold other than one:

```go
validator.Rules{"$atleast": "a|b|c|size:2"}
// {"a":"x"} -> "at least 2 of a, b and c are required"
```

`$atmost` caps instead:

```go
validator.Rules{"$atmost": "mail|phone"}
// both given -> "at most one of mail and phone can be given"
```

Pass a `[]string` to apply several independent groups:

```go
validator.Rules{"$atleast": []string{"a|b", "x|y"}}
```

Counting is by **presence, not truthiness** — `false` counts as present.

## 13. Operators

Operators are **map keys**, exactly as in the npm package. A rule value whose
single key is `$or`, `$and` or `$switch` is an operator node:

```go
validator.Rules{"id": validator.Rules{"$or": []any{"objectid", "uuid"}}}
```

An operator node carries that one key and nothing else. Mixing it with a
sibling key is a rule error, so wrap them in a branch instead.

Because rules are plain maps, an operator tree can be decoded from JSON or
assembled at runtime. A typed builder is available if you prefer it, and
produces the identical node:

```go
validator.Rules{"id": validator.Operator{Or: []any{"objectid", "uuid"}}}
```

### `$or`

Passes if any branch passes. Branches may be any rule shape:

```go
validator.Rules{"id": validator.Rules{"$or": []any{"objectid", "uuid"}}}
```

When every branch fails, `$or` reports the branch that best fits the value, so you get a specific message instead of a vague one. It prefers branches whose type matches the value, then the branch with the fewest errors:

```go
rules := validator.Rules{"address": validator.Rules{"$or": []any{
    "string|max:60",
    validator.Rules{"city": "name", "pin": "string|natural|size:6"},
}}}
// {"address": {"city": "123", "pin": "123456"}}
//   -> "address.city must be a valid name"    (the object branch, not the string one)
```

A branch that is only `optional` or `nullable` is rejected as a rule error — it would accept anything. Use `$and` for that.

### `$and`

Every branch must pass. Its main use is making a nested object or array optional, which is otherwise not expressible:

```go
validator.Rules{"billing": validator.Rules{"$and": []any{
    "optional",
    validator.Rules{"line1": "string|min:5", "city": "name"},
}}}

validator.Rules{"products": validator.Rules{"$and": []any{
    "optional",
    []validator.Rules{{"title": "string|min:5", "price": "positive"}},
}}}
```

`$and` **merges its plain object branches** into one rule set, so a key
declared in any branch counts as declared under `Strict`:

```go
rules := validator.Rules{"v": validator.Rules{"$and": []any{
    validator.Rules{"a": "string"}, validator.Rules{"b": "string"}}}}
// {"v": {"a": "x", "b": "y"}}        -> passes under Strict
// {"v": {"a": "x", "b": "y", "c": 1}} -> "v.c is not required"
```

It also combines a custom function with built-in rules:

```go
validator.Rules{"n": validator.Rules{"$and": []any{"natural", validator.CustomRule(isEven)}}}
```

### `$switch`

Applies one rule, chosen by which `case` passes. The first match wins; evaluation stops there:

```go
validator.Rules{"amount": validator.Rules{"$switch": []any{
    map[string]any{"case": "number|max:1000", "then": "positive", "default": true},
    map[string]any{"case": "number|min:1001", "then": "positive|decimalmax:2"},
}}}
```

Each branch is a map accepting only `case`, `then` and `default`. Both `case` and `then` take any rule value, including functions, object rules, tuple rules and nested operators.

`"default": true` marks the branch used when no case matches. Without one, a non-matching value reports `does not match any case` (`NO_CASE_MATCHED`). At most one branch may be the default.

## 14. Custom rules

A rule value can be a function:

```go
type CustomRule func(value, parent any) *RuleError
```

Return `nil` to pass, or a `*RuleError` to fail. You own the message and the code:

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

`parent` is the enclosing object at that level, which is what makes cross-field checks work without special syntax.

Two things to know:

- **The function runs even when the value is absent**, receiving `nil`. It owns the decision about absence — there is no implicit `is required`. An absent key and an explicit `null` both arrive as `nil`.
- **The code is yours.** It is not restricted to the built-in vocabulary; `"PASSWORD_MISMATCH"` passes straight through to `Detail.Code`. Omit it and it defaults to `CUSTOM_RULE_FAILED`.

### Where a custom rule may appear

Anywhere a rule value is accepted: a top-level field, inside a nested object
rule, inside a tuple rule (running once per element), on a dotted path, on an
indexed or negative-indexed key, on a slice key (running once per selected
element), and as a branch of `$or`, `$and`, or as a `$switch` `case` or `then`.

### The return contract

The npm package accepts any return value and throws on a malformed one — `true`,
a string, or an object missing `message` or `code`. In Go the signature makes
most of those unrepresentable: a custom rule returns `*RuleError` or nothing, so
the compiler rejects them before the validator ever sees them. Two runtime
checks remain:

- Returning a `RuleError` with an **empty `Message`** is a rule error naming the field.
- A **panic** inside a custom rule is recovered and reported as a rule error
  naming the field, rather than crashing the caller — matching npm, which
  converts a thrown error the same way.

A plain func literal works without the `CustomRule` conversion in most positions:

```go
validator.Rules{"n": func(v, _ any) *validator.RuleError { ... }}
```

## 15. Error messages

### `field:` — rename the field

```go
validator.Rules{"age": "natural|field:person age"}
// -> "person age must be a valid natural number"
```

### `error:` — replace the message

```go
validator.Rules{"age": "natural|error:given age is not valid"}
// -> "given age is not valid"
```

The **code is preserved** — only the text changes, so machine consumers still see `NOT_NATURAL`.

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

## 16. Strict mode

Reject fields that have no rule:

```go
validator.Validate(rules, data, validator.Config{Strict: true})
// an unexpected field -> "b is not required"   (UNEXPECTED_FIELD)
```

It applies to nested objects too (`a.c is not required`). Off by default.

## 17. Error ordering

**Error order within one rules level is unspecified.** Go randomizes map iteration, and `Rules` is a map. Ordering between nesting levels is stable, since that follows recursion.

Sort when you need determinism:

```go
sort.Slice(result.Details, func(i, j int) bool {
    return result.Details[i].Field < result.Details[j].Field
})
```

## 18. Duplicate messages

Identical messages collapse, keeping the first occurrence's code. Two fields sharing a custom `error:` text produce one entry — give them distinct messages if you need both.

---

# Reference

Every rule, one entry each. Rules marked **auto** imply a type check, so you need not write `string` or `number` first.

## Presence

### `optional`
Passes when the key is **absent**. Does not accept `nil`.
```go
validator.Rules{"org": "optional|string"}
```

### `nullable`
Passes when the key is present with value `nil`. Does not accept absence.
```go
validator.Rules{"org": "nullable|string"}
```

### `$atleast`
At least N of the named fields must be present. `size:` sets N (default 1). Counts presence, not truthiness. Code: `ATLEAST_NOT_MET`.
```go
validator.Rules{"$atleast": "mail|phone"}
validator.Rules{"$atleast": "a|b|c|size:2"}
validator.Rules{"$atleast": []string{"a|b", "x|y"}}
```

### `$atmost`
At most N of the named fields may be present. Code: `ATMOST_EXCEEDED`.
```go
validator.Rules{"$atmost": "mail|phone"}
```

## Data types

### `string`
Code: `NOT_STRING`. → `f must be string`

### `number`
Code: `NOT_NUMBER`. Accepts every Go numeric kind. With a preceding `string`, validates a numeric string instead (`NOT_NUMERIC_STRING`).
```go
validator.Rules{"a": "number"}         // 42 passes, "42" fails
validator.Rules{"b": "string|number"}  // "42" passes, 42 fails
```

### `boolean`
Code: `NOT_BOOLEAN`. With a preceding `string`, accepts `"true"`/`"false"` (`NOT_BOOLEAN_STRING`).

### `array`
Code: `NOT_ARRAY`. Accepts `[]any` and native Go slices. A string is not an array.

### `object`
Code: `NOT_OBJECT`. Accepts `Data` and `map[string]any`. An array is not an object.

### `bigint`
Code: `NOT_BIGINT`. Accepts `*big.Int` and `json.Number`.

Unlike npm, this survives a JSON round trip. Decode with `UseNumber()` to keep integers beyond 2^53 exact:
```go
dec := json.NewDecoder(r)
dec.UseNumber()
dec.Decode(&data)
validator.Rules{"big": "bigint"} // 12345678901234567890 stays exact
```

## String formats

All **auto** — they imply `string`.

### `email`
`NOT_EMAIL`. → `f must be a valid email`

### `url`
`NOT_URL`. Accepts `https://example.com`, `http://www.example.com`, `www.example.com`. Rejects a bare `example.com`.

### `domain`
`NOT_DOMAIN`. → `example.com`

### `name`
`NOT_NAME`. One or more words: letters, marks, `'`, `’`, `-`, `.`, spaces. Unicode-aware — `José`, `Müller`, `राज`, `北京` all pass.

### `fullname`
`NOT_FULLNAME`. Like `name` but requires **at least two** space-separated parts.

### `username`
`NOT_USERNAME`. 8–20 characters, alphanumeric plus `.` and `_`, no leading or trailing separator, no two consecutive separators. `johndoe123` and `john.doe.x` pass; `john..doe` and `_johndoe` fail.

### `alpha`
`NOT_ALPHA`. `[A-Za-z]+`

### `alphanumeric`
`NOT_ALPHANUMERIC`. `[A-Za-z0-9]+`

### `phone`
`NOT_PHONE`. Optional `+` country code, optional parenthesised group, digits and spaces.

### `phonecode`
`NOT_PHONECODE`. `+` followed by 1–3 digits: `+1`, `+91`.

### `objectid`
`NOT_OBJECTID`. 24 hex characters — a MongoDB ObjectId.

### `uuid`
`NOT_UUID`. `123e4567-e89b-12d3-a456-426655440000`

### `date`
`NOT_DATE`. ISO 8601, permissive: date, optional time, optional zone.

### `dateonly`
`NOT_DATEONLY`. Strictly `YYYY-MM-DD`.

### `time`
`NOT_TIME`. `HH:MM`, `HH:MM:SS`, optional fraction. 24-hour.

### `lower`
`NOT_LOWERCASE`. No uppercase letters. → `f must not contains upper case letters`

### `upper`
`NOT_UPPERCASE`. No lowercase letters.

### `ip`
`NOT_IP`. IPv4 dotted quad.

## Number types

All **auto** — they imply `number`. Each has a numeric-string variant when preceded by `string`, with the same code and a `numeric string` message.

### `int`
`NOT_INTEGER`. No fractional part.

### `positive`
`NOT_POSITIVE`. `> 0`

### `negative`
`NOT_NEGATIVE`. `< 0`

### `natural`
`NOT_NATURAL`. Integer `> 0`

### `whole`
`NOT_WHOLE`. Integer `>= 0`

## Constraints

### `equal:`
Compares against the value's own type — `equal:200` matches both `200` and `"200"`. Booleans compare against `true`/`false`. Code: `NOT_EQUAL`.

### `size:`
Overloaded by runtime type:

| Value | Meaning | Code |
|---|---|---|
| string | rune length | `LENGTH_MISMATCH` |
| array | element count | `LENGTH_MISMATCH` |
| number | digit count, ignoring sign and point | `DIGITS_MISMATCH` |

```go
validator.Rules{"a": "string|size:6"}  // "123456"
validator.Rules{"b": "array|size:2"}   // [1, 2]
validator.Rules{"c": "number|size:3"}  // 123, -123 and 1.23 all pass
```

### `min:`
Overloaded four ways:

| Value | Meaning | Code |
|---|---|---|
| string | minimum length | `TOO_SHORT` |
| array | minimum element count | `TOO_SHORT` |
| number | minimum value | `TOO_SMALL` |
| date (non-numeric argument) | not before | `DATE_TOO_EARLY` |

```go
validator.Rules{"d": "date|min:2024-01-01"}
// "2023-06-01" -> "d must be at least 2024-01-01T00:00:00.000Z"
```

### `max:`
The mirror of `min:` — `TOO_LONG`, `TOO_LARGE`, `DATE_TOO_LATE`.

### `regex:`
JavaScript literal or Go inline-flag form. Implies `string`. Code: `REGEX_MISMATCH` → `f is invalid`. See [Guide §8](#8-regular-expressions) for the RE2 restriction.

### `decimalsize:`
Exactly N digits after the point. Code: `DECIMAL_SIZE_MISMATCH`.

Numeric strings preserve trailing zeroes; floats cannot:
```go
validator.Rules{"a": "string|number|decimalsize:2"} // "10.50" passes
validator.Rules{"b": "number|decimalsize:3"}        // 23.340 fails — the float is 23.34
```

### `decimalmin:`
At least N decimal places. `DECIMAL_TOO_FEW`.

### `decimalmax:`
At most N decimal places. `DECIMAL_TOO_MANY`.

### `enums:`
Comma-separated allowed values. Branches on the value's runtime type — strings compare literally, numbers numerically, booleans against `true`/`false`. Code: `ENUM_MISMATCH` → `f is invalid`.
```go
validator.Rules{"role": "enums:admin,user,guest"}
validator.Rules{"n": "number|enums:1,2,3"}
```

### `field:`
Renames the field in messages. Must come last. Suppressed for keys ending in an array index.

### `error:`
Replaces the message, preserving the code. Must come last.

## Arrays

### `arrayof:<rule>`
Applies any rule above to every element, labelling by index. Nests arbitrarily. `arrayof:optional` and `arrayof:nullable` allow absent elements. A non-array fails with `NOT_ARRAY`.

## Operators

### `$or`
Passes if any branch passes. Reports the best-fitting branch on failure: type-matching branches first, then fewest errors. A branch that is only `optional`/`nullable` is a rule error.

### `$and`
Every branch must pass. Use with `optional` to make a nested object or tuple rule optional.

### `$switch`
One rule chosen by the first passing `case`. `"default": true` marks the fallback. Without one: `NO_CASE_MATCHED`.

---

# Error codes

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

A custom rule's code is not restricted to this list.

---

# Differences from the npm package

Full detail in [LIMITATIONS.md](LIMITATIONS.md). In summary:

**Rule vocabulary: 31 of npm's 33 bare rules, and all 11 argument rules.** The two omissions:

- **`symbol`** — a JavaScript-only identity primitive with no Go or JSON counterpart. Reported as an unknown rule.
- **`mongoid`** — npm's own deprecated alias for `objectid`, which emits a deprecation warning there. Since this package starts fresh at v0.1.0, only `objectid` exists.

**Everything else is present**: all data types, string formats, number types, constraints, `arrayof:`, the three operators, `$atleast`/`$atmost`, indexing and slices, custom rules, `field:`/`error:`, and all three config options.

**Behavioural differences:**

| | npm | Go |
|---|---|---|
| Data input | JS object | `map[string]any` only (no structs) |
| Error order | rules declaration order | unspecified within a level |
| `regex:` | full JS engine | RE2: no lookahead, lookbehind or backreferences |
| `bigint` | cannot survive JSON | `*big.Int` / `json.Number`, exact |
| Malformed rule | throws | returned as `error` |
| Field path | parse from message | `Detail.Field` |

---

# Testing

```sh
go test ./...                  # run the suite
go test -v ./...               # per-test output
go test -race -count=2 ./...   # concurrency check
go test -cover ./...           # coverage
```
