<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-hcl2/brand/main/social/go-ruby-hcl2-hcl2.png" alt="go-ruby-hcl2/hcl2" width="720"></p>

# hcl2 — go-ruby-hcl2

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-7B42BC)](https://go-ruby-hcl2.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo), from-scratch HCL2 (HashiCorp Configuration Language v2)
parser and evaluator for the Ruby value model.** `Parse` turns an HCL2 document
into a lazy `*Body` (attributes and blocks, expressions unevaluated); `Eval`
parses *and* evaluates a document against a variables/functions context, handing
back a Ruby `Hash` (an insertion-ordered `*Map`) — so it is the deterministic,
interpreter-independent core of `HCL2.parse` / `HCL2.eval`, **without any Ruby
runtime**.

It is the HCL2 backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych engine),
[go-ruby-toml](https://github.com/go-ruby-toml/toml) (the toml-rb engine) and
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine).

> **Faithfulness.** There is no canonical Ruby HCL gem to mirror, so this package
> is faithful to the **HCL2 native-syntax specification** (the `hashicorp/hcl` v2
> grammar), not to a gem. It is a clean-room pure-Go implementation mirroring the
> structure and semantics of the org's from-scratch C reference,
> [`libhcl/c-hcl2`](https://github.com/libhcl/c-hcl2); **nothing from
> `hashicorp/hcl` is vendored** — *être capable de compiler depuis les sources est
> un gage d'indépendance.*

## Features

Faithful, from-scratch implementation of the HCL2 **native syntax**:

- **Bodies** — attributes (`name = expr`) and blocks (`type "label" "label2"
  { … }`) with nested bodies; blocks of the same type **collect**, labels are
  preserved (each label nests one level), newline-terminated items, and `#` /
  `//` / `/* … */` comments.
- **Literals** — numbers (integral → `Integer`, fractional/exponent → `Float`),
  `true` / `false` / `null`, strings, tuples `[…]`, and objects `{ k = v, "k2"
  = v }` (bare-identifier or quoted/computed keys, `=` or `:` separators).
- **Strings & templates** — `${ … }` interpolation (brace-depth aware), the
  `$${` / `%%{` literal escapes, `\n \t \r \" \\` and `\uXXXX` / `\UXXXXXXXX`
  escapes, `%{ if … }…%{ else }…%{ endif }` and `%{ for … }…%{ endfor }`
  directives (nesting), and **heredocs** `<<EOT` / `<<-EOT` (indent strip,
  backslashes kept literal, interpolation supported).
- **Traversal** — attribute access `a.b.c`, index `a[0]` / `a["k"]`, the `a.0`
  numeric-attribute sugar, and **splats** `a.*.b` / `a[*].b` (desugared to a
  tuple `for`-expression; chained splats rejected).
- **Operators** — arithmetic `+ - * / %` (with string `+` concatenation),
  comparison `< <= > >=`, equality `== !=` (deep, number-cross-type),
  logical `&& ||` (short-circuit), unary `-` / `!`, and the right-associative
  conditional `c ? t : f` — all with HCL's precedence.
- **`for`-expressions** — tuple `[for x in xs : e if cond]` and object
  `{for k, v in m : k => v if cond}`, the optional key/index var, the `if`
  filter, and the `…` **grouping mode** for objects.
- **Function calls** — `f(args…)` with the trailing `…` **spread**, the `try` /
  `can` special forms (lazy, error-suppressing), and a pure standard-library
  subset (below).
- **Diagnostics** — `Diagnostic` / `Diagnostics` with 1-based `line:col`
  positions, mirroring HCL's error reporting.

CGO-free, dependency-free, **100% test coverage**, `gofmt` + `go vet` clean, and
green across the six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le,
s390x) on Linux, macOS, and Windows.

## Install

```sh
go get github.com/go-ruby-hcl2/hcl2
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-hcl2/hcl2"
)

func main() {
	ctx := hcl2.NewContext()
	ctx.Variables["env"] = "prod"

	m, _ := hcl2.Eval(`
name    = "web-${env}"
count   = 3
ports   = [for p in [80, 443] : p]
service "api" {
  port = 9090
}
`, ctx)

	// Eval returns an insertion-ordered *hcl2.Map (a Ruby Hash).
	name, _ := m.Get("name")
	fmt.Println(name) // web-prod

	svc, _ := m.Get("service")
	api, _ := svc.(*hcl2.Map).Get("api")
	port, _ := api.(*hcl2.Map).Get("port")
	fmt.Println(port) // 9090

	// Parse alone yields a lazy *Body for per-attribute decoding.
	body, _ := hcl2.Parse(`region = upper("eu")`)
	region, _, _ := body.Attr("region", nil)
	fmt.Println(region) // EU
}
```

## Ruby value model

`Eval` returns an `any` drawn from a small, fixed set of Go types, so a host can
map its own object graph to and from this package:

| HCL2                | Go (`Eval` returns)     | Ruby (`rbgo` maps to) |
| ------------------- | ----------------------- | --------------------- |
| string / template   | `string`                | `String`              |
| number (integral)   | `int64`                 | `Integer`             |
| number (fractional) | `float64`               | `Float`               |
| bool                | `bool`                  | `true` / `false`      |
| null                | `nil`                   | `nil`                 |
| tuple               | `[]any`                 | `Array`               |
| object              | `*hcl2.Map` (ordered)   | `Hash`                |

HCL has a single number type; this package narrows an integral number to `int64`
and a fractional one to `float64`, so the host materialises `Integer` vs `Float`
naturally — exactly as it does for the JSON and TOML backends.

## API

```go
// Parse parses a document into a lazy *Body (no evaluation).
func Parse(src string) (*Body, error)

// Eval parses and evaluates a document against ctx, returning a Ruby Hash.
func Eval(src string, ctx *Context) (*Map, error)

// EvalExpr parses and evaluates a single expression string.
func EvalExpr(src string, ctx *Context) (Value, error)

// EvalFile reads and evaluates a file.
func EvalFile(path string, ctx *Context) (*Map, error)

// Body.Attr evaluates one attribute lazily against ctx.
func (b *Body) Attr(name string, ctx *Context) (Value, bool, error)

type Context struct {
	Variables map[string]any
	Functions map[string]Func
}
func NewContext() *Context

type Func func(args []any) (any, error)

type Map struct { /* insertion-ordered Hash */ }
func NewMap() *Map
func (m *Map) Set(key string, val any)
func (m *Map) Get(key string) (any, bool)
func (m *Map) Pairs() []Pair
func (m *Map) Len() int

type Diagnostic  struct{ Summary string; Pos Pos } // mirrors hcl.Diagnostic
type Diagnostics []*Diagnostic                      // mirrors hcl.Diagnostics
type Pos         struct{ Line, Col, Offset int }
```

## Standard-library functions

A pure (side-effect-free) subset of the Terraform/HCL function library ships
built-in; a host shadows any of them via `Context.Functions`:

`length`, `upper`, `lower`, `trimspace`, `min`, `max`, `abs`, `floor`, `ceil`,
`concat`, `join`, `split`, `keys`, `values`, `lookup`, `contains`, `coalesce`,
`reverse`, `format`, `tostring`, `tonumber`, `tobool`, `jsonencode`,
`jsondecode`, plus the `try` / `can` special forms.

`format` supports the `%s %d %f %g %t %q %v %%` verbs. `keys` / `values` return
their results in sorted-key order (matching HCL). `jsonencode` emits objects in
insertion order; `jsondecode` round-trips into the value model above.

## Grammar coverage

| Area | Status |
| ---- | ------ |
| Bodies (attributes, blocks, labels, comments) | ✅ implemented |
| Literals (number/bool/null/string/tuple/object) | ✅ |
| Interpolation `${…}`, `$${`/`%%{` escapes, unicode escapes | ✅ |
| Heredocs `<<EOT` / `<<-EOT` (indent strip) | ✅ |
| Template directives `%{ if }` / `%{ for }` (nesting) | ✅ |
| Traversal (`.attr`, `[idx]`, `a.0` sugar) | ✅ |
| Operators (arith/compare/equality/logical/unary/conditional) | ✅ |
| `for`-expressions (tuple + object, `if`, grouping `…`) | ✅ |
| Splats (`a.*.b`, `a[*].b`) | ✅ |
| Function calls + `…` spread + `try`/`can` | ✅ |
| **Deferred** — `%{~ … ~}` whitespace-strip markers | ⏳ documented |
| **Deferred** — the JSON-syntax profile (HCL JSON variant) | ⏳ documented |
| **Deferred** — the full `cty` type-conversion system | ⏳ documented |

The deferred items are deliberately scoped out of this milestone (matching the
C reference's roadmap) and are documented here rather than faked; the native
syntax above is complete.

## Tests & coverage

There is no Ruby HCL gem oracle, so the whole suite is **deterministic,
interpreter-free** spec-vector + golden tests (parse-tree and eval-result
assertions, plus an error/diagnostic corpus). They alone hold coverage at 100%,
so every CI lane — Linux/macOS/Windows and the qemu cross-arch lanes — passes the
gate identically.

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # 100.0%
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-hcl2/hcl2 authors.
