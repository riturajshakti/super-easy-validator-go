# Limitations of the Go port

This document records what the Go version of `super-easy-validator` cannot
reproduce from the npm package, and what it does instead. It is deliberately
short: most of what looked like a limitation during the initial survey turned
out to be an internal implementation detail invisible to callers, a property of
an input format that has since been dropped, or something avoidable by a
versioning decision.

Findings were verified against Go 1.26.5 by compiling the actual patterns and
running the actual conversions, not by inspection.

Reference implementation: the npm package `super-easy-validator` v0.9.0
(`../super-easy-validator`).

---

## 1. Regex features that Go's engine does not have

**Severity: permanent, but narrow. Affects the `regex:` rule only.**

This is the one genuine limitation in the package.

**Rule syntax is identical to npm.** Patterns are written in JS literal form,
`regex:/^[A-Z0-9]{128}$/i`, and translated to Go's inline-flag form
automatically. Callers do not write Go-specific syntax:

```
/^[A-Z0-9]{128}$/i    ->  (?i)^[A-Z0-9]{128}$
/^a.b$/is             ->  (?is)^a.b$
/^\d+$/               ->  ^\d+$
/^x$/gu               ->  ^x$
```

`i`, `m` and `s` map to `(?i)`, `(?m)`, `(?s)`. JS `g` and `y` are dropped —
they are stateful-iteration flags, meaningless for the full-string test this
rule performs. `u` is dropped because Go patterns are UTF-8 aware by default.
A pattern already written in Go form passes through untouched, so both spellings
work.

**What translation cannot fix is the engine.** Go's `regexp` implements RE2,
which guarantees linear-time matching by not implementing backtracking at all.
Lookahead, lookbehind and backreferences are absent from the engine, not merely
spelled differently, so no amount of string rewriting produces an equivalent:

```
/^(?!www)x$/   REJECT  (?! negative lookahead is not supported by Go's regexp engine
/^(a)\1$/      REJECT  backreference \1 is not supported by Go's regexp engine
```

Such a pattern is reported as an invalid-rule error naming the field and the
unsupported construct, rather than silently never matching.

In practice most validation patterns — character classes, anchors, quantifiers,
alternation, Unicode property escapes — are unaffected. Note also that this
applies *only* to caller-supplied patterns: every built-in format rule,
including `username` and `url` whose npm implementations use lookahead, behaves
identically to npm. See "Implementation notes" below.

## 2. Four behaviours differ deliberately from npm

Each of these was checked against the npm package and the Go behaviour chosen
on purpose. They are pinned by tests so they cannot drift silently.

### String length counts runes, not UTF-16 code units

JavaScript's `.length` counts UTF-16 code units, so an astral-plane character
such as an emoji counts as two. Go counts runes, so it counts as one:

```
emoji with size:1   npm fails   Go passes
emoji with size:2   npm passes  Go fails
```

Counting runes is the meaning a Go caller expects from "length", and it makes
`size:`, `min:` and `max:` agree with `utf8.RuneCountInString`. Characters in
the Basic Multilingual Plane, which is nearly all text, count the same in both.

### NaN and Infinity are not integers

npm's integer rules test for a decimal point in the value's string form, so
`NaN` and `Infinity` slip through `natural`, `whole` and `int`. Go tests the
value, so they are rejected:

```
NaN with natural        npm passes  Go fails
Infinity with natural   npm passes  Go fails
```

Both remain valid for the plain `number` rule in either language, and `NaN`
still fails the decimal rules with `NOT_A_NUMBER`.

### A dotted path asserts its intermediates are objects

A rule key of `a.b.c` says three things: `a` is an object, `a.b` is an object,
and `a.b.c` satisfies the rule. When an intermediate holds something else, Go
reports **that segment**, because that is the value which is actually wrong:

```
{"a": 5}         with rule "a.b.c"   ->  "a must be of type object"
{"a": {"b": 5}}  with rule "a.b.c"   ->  "a.b must be of type object"
{"a": "xy"}      with rule "a.b"     ->  "a must be of type object"
```

npm instead keeps the last object it reached and re-reads the segment it just
rejected, so `{"a": 5}` with `a.b.c` resolves to `5` and reports
`"a.b.c must be string"` — a message naming a path the engine never looked at.
The same quirk lets `{"a": "xy"}` satisfy `a.b`, passing a path that does not
exist, and exposes JavaScript internals such as `String.length` through rule
keys like `a.length`.

Reporting the broken segment makes the error point at the value to fix, and
keeps a dotted rule consistent with the equivalent nested rule: `{"a": {"b":
"string"}}` and `{"a.b": "string"}` now report the same thing for `{"a": 5}`.

`optional` does not suppress this. It excuses a field that is **absent**; an
`a` that is present and is not an object is a type error, not an absence. A
nil intermediate still counts as absent, so `nullable` data reads as missing.

### A malformed custom rule is returned, not thrown

Go's type system rejects most of npm's malformed custom-rule returns at compile
time, since a `CustomRule` returns `*RuleError` or nothing. The two that remain
— an empty `Message`, and a panic inside the function — are reported as rule
errors naming the field. See the custom rule section of DOCS.md.

## 3. Float formatting differs at magnitude extremes

**Severity: edge-case behavior difference.**

Digit-counting rules (`size:`, `decimalsize:`, `decimalmin:`, `decimalmax:`)
operate on a number's string form, and Go and JavaScript disagree at the
extremes of the range:

```
1e21   JS "1e+21"   Go "1000000000000000000000"
```

Within ordinary magnitudes the two agree, so this affects only very large or
very small values. `NaN` remains representable and keeps its npm behavior:
valid as a `number` for type purposes, but failing decimal constraints with the
`NOT_A_NUMBER` code.

The npm caveat that `number` loses trailing zeroes (`23.34` cannot be
distinguished from `23.340`) applies identically in Go, for the same reason —
it is a property of binary floating point, not of either language. Numeric
strings (`'string|number|decimalsize:2'` against `"10.50"`) preserve trailing
zeroes in both.

## 4. Go registry mechanics

**Severity: process differences, not functional limits.**

There is no publish step and no registry account. `pkg.go.dev` serves modules
straight from public git tags, which imposes:

- **The module path must equal the repository URL.** This module is
  `github.com/riturajshakti/super-easy-validator-go`, so it must live in a repo
  at that exact address. This is why the Go port is a separate repository from
  the npm package rather than a subdirectory of it — a Go module at the npm
  repo root would collide with the existing `package.json` tooling and share a
  tag namespace with the npm releases.
- **Versioning is independent of npm.** The Go package starts at `v0.1.0` and
  carries its own tags in its own repo. It does not track the npm package's
  `0.9.0`.
- **Versions are immutable.** Once a version is tagged and fetched, the module
  proxy caches it permanently. A bad release is corrected by publishing a new
  version, never by retagging.
- **Tags are `v`-prefixed and must be semver.**
- **Publication is effectively irreversible.** The proxy and checksum database
  retain what they have fetched, so a module path is a long-term commitment.

### The `/v2` suffix, and how this package avoids it

Go enforces *semantic import versioning*: the major version is part of the
module's identity, so `v2.0.0` and later must carry a `/vN` suffix on the
module path. This is enforced by the toolchain, not by convention — requiring a
`v2` without the suffix fails outright:

```
go.mod:3: require <module>: version "v2.0.0" invalid: should be v0 or v1, not v2
```

The rule in full:

| Tagged version | Required module path |
|---|---|
| `v0.x.x` | `github.com/riturajshakti/super-easy-validator-go` |
| `v1.x.x` | `github.com/riturajshakti/super-easy-validator-go` |
| `v2.0.0`+ | `github.com/riturajshakti/super-easy-validator-go/v2` |

**This package avoids the suffix permanently by never tagging a v2.** The
constraint only activates at `v2.0.0`; `v0` and `v1` are exempt, so starting at
`v0.1.0` means the import path stays
`github.com/riturajshakti/super-easy-validator-go` indefinitely. Breaking
changes can ship as `v0.x` minor bumps — within `v0`, Go already treats every
minor as potentially breaking — or, after a `v1.0.0`, as additive changes.
Reaching `v2` would have to be a deliberate decision, and is not planned.

---

# Design notes

These are not limitations. They record decisions where Go and JavaScript differ
enough that the choice is worth writing down.

## Input is `map[string]any` only

The package validates `map[string]any` — the direct analogue of the JavaScript
object it was designed around. Struct input was considered and dropped.

The reason is that structs cannot represent the package's presence model.
`optional` keys off *absent* and `nullable` keys off *present-but-null*, and the
npm tests assert the two are not interchangeable. A map models this natively:

```
map:  a present=true value=<nil>   (null)
      c present=false              (absent)
```

`encoding/json` decoding into a struct collapses both to the same nil pointer:

```
struct: A=<nil> C=<nil>   (null and missing both nil)
```

Supporting structs would therefore mean shipping a second input path with
weaker semantics, plus a reflection layer to drive it. Restricting to maps keeps
one code path with exact npm fidelity. Callers holding a struct marshal it to a
map first.

## Numbers: one int, one float

Go has eleven numeric types; JavaScript has one. Exposing that at the API
surface would make callers reason about bit widths to validate a phone number,
so the wrapper collapses it: **integer rules (`int`, `natural`, `whole`) work in
integers, and `number` and the decimal rules work in floats.** No `int32`,
`float32` or similar appears in any user-facing type or message.

Internally the widest types are used throughout — `int64` and `float64` — so
range is never the binding constraint. Every Go numeric kind normalizes to that
pair before any rule runs, so a caller may pass `int`, `int8`…`int64`,
`uint`…`uint64`, `float32` or `float64` and get identical results:

```
int      5    -> int=5  float=5    isInt=true
int32    5    -> int=5  float=5    isInt=true
uint8    5    -> int=5  float=5    isInt=true
float64  42   -> int=42 float=42   isInt=true    <- integer rules apply
float64  7.5  -> int=7  float=7.5  isInt=false   <- they do not
```

Integer-ness is decided by value, not by static type, matching the npm package
— which tests for a decimal point in the string form rather than inspecting a
type. So a JSON `7` (which decodes to `float64`) satisfies `natural`, exactly
as it does in JavaScript.

Normalizing to the widest float at the boundary is also what keeps narrow input
safe. A `float32` carried through unwidened would leak artifacts the caller
never wrote:

```
float32(2.3) widened to float64 -> 2.299999952316284
```

That value would fail `decimalsize:1` for reasons invisible in the source. The
normalizer widens once, at the boundary, and formats at a single precision
thereafter.

## Large integers and `bigint`

Plain `encoding/json` decoding into `any` destroys precision past 2^53 — the
same limit as JavaScript, but reached silently:

```
{"big": 12345678901234567890}  ->  1.2345678901234567e+19
```

Decoding with `UseNumber()` preserves the literal, and `*big.Int` reconstructs
it exactly:

```
json.Number: 12345678901234567890
*big.Int:    12345678901234567890   exact=true
```

The npm `bigint` rule is therefore **implemented rather than dropped**, backed
by `*big.Int` and accepting `json.Number` input. This is a genuine improvement
over the npm package, where `bigint` values cannot survive a JSON round trip at
all.

The `symbol` rule has no counterpart — it is a JavaScript-only identity
primitive with nothing to model it in Go or JSON — and is omitted. Using it is
reported as an unknown rule.

## Error ordering is unspecified within a rules level

The npm test suite pins error order to the declaration order of the rules
object, relying on JavaScript's insertion-order guarantee for string keys. Go
randomizes map iteration:

```
run 0: [c d e a b]
run 1: [c d e a b]
run 2: [a b c d e]
```

Rather than complicate the public API to defend an ordering that callers should
not depend on in either language, `Rules` stays a plain map and **error order
within one rules level is unspecified**. Relative ordering between nesting
levels is still stable, since that follows recursion structure rather than map
iteration. Callers needing deterministic output should sort by the `field` of
each returned detail.

## Implementation notes on built-in patterns

Two npm built-ins use lookahead, which RE2 rejects. Both are reimplemented with
identical observable behavior, so callers writing `'username'` or `'url'` see
no difference:

- **`username`** — the lookahead `(?!.*?[._]{2})` asserts "no two consecutive
  `.` or `_`". That is a rejection test rather than a matching constraint, and
  is expressed as a compiled pattern plus a substring check. Verified against
  accepting `johndoe123` and `john.doe.x`, and rejecting `john..doe`,
  `john._doe`, `john__doe`, `_johndoe`, `johndoe_` and over-short input.
- **`url`** — the alternation `(?:www\.|(?!www))` is redundant: the left branch
  accepts a `www.` prefix and the right asserts its absence, so together they
  constrain nothing and the group is dropped. Verified against accepting
  `https://example.com`, `http://www.example.com`, `www.example.com` and
  `https://sub.example.co.uk`, and rejecting `example.com` and non-URLs.

The rejected alternative was depending on `github.com/dlclark/regexp2`, which
supports lookahead with exact JS semantics. It was rejected because
zero-dependency is a stated selling point, and a backtracking engine would
reintroduce the ReDoS exposure the npm suite has explicit timing tests against
(`test/names.test.js`).

---

## What ports without compromise

The core design survives intact: string rules (`'optional|email'`) and both
separator forms; nested object rules and single-element tuple rules for arrays
of objects; recursive `arrayof:`; the `$or`, `$and` and `$switch` operators
including `$or`'s branch-scoring algorithm; `$atleast` and `$atmost`; bracket
and slice key indexing with negative indices; custom function rules as
`func(value, parent any) *RuleError`; `field:` and `error:` overrides; the
`quotes` and `strict` options; every built-in format rule; JS-style
`regex:/pat/flags` syntax; the full error-code vocabulary; and the
`{errors, details}` result shape with its index alignment and message-level
de-duplication.
