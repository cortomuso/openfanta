# Rule/Expression Languages for Deterministic, User-Configurable Fantacalcio Scoring (Go backend)

Research date: 2026-10-27.

## Candidate comparison: determinism, termination, sandboxing, arithmetic, typing, maturity, ports, spec stability

### Takeaway
CEL is the strongest fit for an escape-hatch expression language. Its spec guarantees determinism and termination, it has side-effect-free semantics, integer overflow is an error rather than a silent wrap, it supports static type checking, cost units are deterministic, and the Go library is actively released (v0.33.0, Oct 2026) with official C++, Java and Python implementations plus a TS evaluator from Buf. Expr is a capable Go-only alternative but has no spec and no ports. Starlark gives exact big integers and insertion-ordered dicts, but it is a full language that needs step limits. JSONLogic has many ports but no formal spec, and its implementations disagree on edge cases. Rego, gopher-lua and Wasm each have a determinism or arithmetic problem that rules them out as the primary choice.

### Cited Findings

**CEL (cel-spec / cel-go)**
- Spec: "For a given evaluation environment, a CEL expression will deterministically evaluate to either a value or an error." Sub-expression evaluation order is not guaranteed. — [cel-spec langdef](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- Spec: CEL is "terminating: CEL programs cannot loop forever." Macros (other than `has()`) are "the only avenue for exponential behavior." — [cel-spec langdef](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- Spec: only 64-bit int/uint and IEEE-754 double. "Arithmetic operations raise an error when the results exceed the range of the integer type." There is no decimal type. — [cel-spec langdef](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- Spec: "a CEL program only computes an output from its inputs"; "CEL functions have no observable side effects". The type checker is optional and "does not affect the result of evaluation". The spec defines time and space complexity for constructs, and extension authors are responsible for the cost of their own functions. — [cel-spec langdef](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- The spec does not say anything about map iteration order. — [cel-spec langdef](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- Durations cover roughly ±290 years. Time is only available if the host passes it in as a variable; no clock is mentioned in the spec. — [cel-spec langdef](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- cel-go has moved to github.com/cel-expr/cel-go. The old google/cel-go repo is a "READ-ONLY COMPATIBILITY REPOSITORY" that "will eventually be removed". — [google/cel-go](https://github.com/google/cel-go)
- Recent releases. v0.33.0 (Oct 26) added a Pratt parser, a unified cost model and runtime peak-memory tracking; it is breaking because `CostModelVersion(1)` is now the default. v0.32.0 (Aug 19) changed the module path to `cel.dev/cel-go`. v0.31.0 (Aug 7) added size limits for compiled regex. v0.30.0 (Jul 26) added expression node limits for the parser and checker. — [cel-expr/cel-go releases](https://github.com/cel-expr/cel-go/releases)
- Kubernetes: cost units "are independent of system load and hardware… deterministic; for any given CEL expression and input data… will always result in the same cost". Comparisons cost 1 and list literals cost 40. Kubernetes enforces a runtime cost budget per expression and per validation, and it rejects at write time any expression whose estimated worst-case cost is too high. — [Kubernetes CEL docs](https://kubernetes.io/docs/reference/using-api/cel/)
- Kubernetes gates CEL library features by version. Writes are accepted only on API servers that support the feature, and "when a cluster is rolled back… CEL expressions… already stored… will continue to evaluate correctly". Examples: optional types from 1.29, TwoVarComprehensions from 1.33. — [Kubernetes CEL docs](https://kubernetes.io/docs/reference/using-api/cel/)
- Implementations in other languages:
  - Official Python: CEL-expr-python was open-sourced in March 2026. It wraps cel-cpp and is maintained by the CEL team. — [Google Open Source Blog](https://opensource.googleblog.com/2026/03/announcing-cel-expr-python-the-common-expression-language-in-python-now-open-source.html)
  - Rust: cel-rust (community) and cel-cxx (bindings to cel-cpp). Java: several ports. — [awesome-cel](https://github.com/datreeio/awesome-cel), [cel-cxx](https://docs.rs/cel-cxx)
  - TS/JS: `@bufbuild/cel` is an ECMAScript evaluator, and `@bufbuild/cel-spec` ships the upstream conformance data (pinned to spec v0.25.1). — [@bufbuild/cel-spec README](https://cdn.jsdelivr.net/npm/@bufbuild/cel-spec@0.4.0/README.md)
- cel-spec release 0.25.1 is dated Nov 13 2025, and 0.24.0 is dated 2025-06-26. — [Bazel Central Registry](https://registry-preview.bazel.build/modules/cel-spec)

**Expr (expr-lang/expr)**
- The README describes it as side-effect free, designed to prevent infinite loops, and statically type-checked before runtime (example: `name + age` fails at compile time). Listed users include Google, Uber, GoDaddy, ByteDance, Alibaba and OpenTelemetry. 8.1k stars. — [expr-lang/expr](https://github.com/expr-lang/expr)
- Releases: v1.17.0 added "compile node budget and memory limits"; v1.17.5 added `expr.MaxNodes()`; v1.17.7 (Dec) added recursion-depth limits in builtins and stack-underflow fixes; v1.17.8 (Feb 14) added a bounds check in `get()`. — [expr releases](https://github.com/expr-lang/expr/releases)
- CVE-2025-29786 (GHSA-93mq-9ffx-83m2, CVSS 7.5, published 2025-03-17): an unbounded expression size let the parser build a huge AST and crash the process with OOM. It was fixed in 1.17.0 with compile-time limits on node count and memory. — [GitLab advisory](https://advisories.gitlab.com/pkg/golang/github.com/expr-lang/expr/CVE-2025-29786/)
- I found no formal language spec and no implementations in other languages on the README. — [expr-lang/expr](https://github.com/expr-lang/expr)

**Starlark (go.starlark.net)**
- Spec: "there are no sources of random numbers, clocks, or unspecified iterators". Dicts and sets iterate "in the order in which they were inserted". Module globals are frozen after execution. — [starlark-go spec](https://raw.githubusercontent.com/google/starlark-go/master/doc/spec.md)
- Integers "may be… arbitrarily large" and "Integer arithmetic is exact". Floats are IEEE 754. Division by zero is a dynamic error. — [starlark-go spec](https://raw.githubusercontent.com/google/starlark-go/master/doc/spec.md)
- `while` loops and recursion are off by default and need the `-recursion` flag. However, `for` loops over large ranges are still possible, so termination is not structurally guaranteed. — [starlark-go spec](https://raw.githubusercontent.com/google/starlark-go/master/doc/spec.md)
- `Thread.SetMaxExecutionSteps` caps abstract steps, a measure that is "deterministic and reproducible, but it does not correspond to CPU time". The Go implementation "strives to match" the Java implementation used by Bazel. No tagged releases are shown. — [google/starlark-go](https://github.com/google/starlark-go)

**JSONLogic**
- Design: rules are data and the library "never eval()"s them. Rules have read-only access to provided data, "no side effects and deterministic computation time". There is no loop construct. Parsers are listed for JS, PHP, Python, Ruby, Go, Java, .NET and C++. The site has no versioning statement and does not specify a number type. — [jsonlogic.com](https://jsonlogic.com/)
- Implementations diverge:
  - Truthiness of empty arrays differs between PHP and JS. — [json-logic community spec page (mirror)](https://verifico.duckdns.org/json-logic)
  - `==` and `===` differ in the Python port. — [json-logic on PyPI](https://pypi.org/project/json-logic)
  - python-jsonlogic says the format "lacks a formal specification" and that dot-notation access is ambiguous. — [python-jsonlogic](https://pypi.org/project/python-jsonlogic)
  - A community formal-spec and test-suite effort exists, but I could not confirm its status. — [json-logic community spec page (mirror)](https://verifico.duckdns.org/json-logic)
- json-everything warns that overriding built-ins breaks interoperability. — [json-everything docs](https://docs.json-everything.net/logic/basics/)

**Rego / OPA**
- Rego has nondeterministic builtins: `time.now_ns`, `rand.intn`, `http.send`, `uuid.rfc4122`, `net.lookup_ip_addr`, `opa.runtime`. For Wasm targets the host SDK must provide them. The builtins page does not document how to disable them; capabilities files would be the mechanism, but I did not verify this. — [OPA builtins](https://www.openpolicyagent.org/docs/policy-reference/builtins)
- In August 2025 Apple hired OPA's core maintainers (Hinrichs, Sandall, Koponen). OPA stays a CNCF graduated project with no change to governance or licensing. Styra's commercial products are being open-sourced, and Styra DAS is being sunset. — [Oso blog](https://osohq.com/post/opa-maintainers-join-apple-oss-community-to-maintain-styra-products), [Open Source For U](https://www.opensourceforu.com/2025/08/apple-acquires-open-policy-agent-developers-while-cncf-retains-control-of-open-source-project/)
- A secondary source reports releases continuing from v1.8.0 (2025-08-28) to v1.13.0 (2026-01-29). I did not verify this. — [youngju.dev review](https://www.youngju.dev/transcribe/2026-07-17-opa-post-styra-apple-acquihire-reality.en)

**Lua via gopher-lua**
- It implements Lua 5.1 plus `goto`. Numbers are `float64` only. Sandboxing works by setting `SkipOpenLibs` and opening selected libraries. Timeouts use `L.SetContext(ctx)`, which the README says degrades performance. 61 open issues and 59 open PRs; no releases shown. — [yuin/gopher-lua](https://github.com/yuin/gopher-lua)

**WebAssembly**
- Wasm is deterministic except for NaN payloads, resource exhaustion and host functions. — [Wasm-R3 paper](https://arxiv.org/pdf/2409.00708), [WebAssembly nondeterminism](https://www.webassembly.org/docs/nondeterminism/)
- Wasmtime can canonicalize NaNs, at some overhead. — [Wasmtime deterministic execution](https://docs.wasmtime.dev/examples-deterministic-wasm-execution.html)
- wasmi has an opt-in feature for the "Wasm 3.0 deterministic profile". — [wasmi source](https://docs.rs/crate/wasmi/2.0.0-beta.8/source/src/lib.rs)
- DFINITY disables threads and SIMD and canonicalizes NaNs. — [DFINITY forum](https://forum.dfinity.org/t/how-is-deterministic-execution-of-webassembly-ensured/6335)

### Inferences
- Integer hundredths fit CEL's int64 well: ±9.2e18 hundredths is far beyond any score, and overflow is an error rather than a silent wrap. Avoid `double` entirely. The Go environment can simply not declare double-producing functions, or the server can reject expressions whose type-checked result type is not `int`.
- Integer division and modulo semantics (truncation toward zero) need pinning in the protocol for every language. CEL specifies them, but JSONLogic does not.
- Gopher-lua and JSONLogic have float-only or unspecified numerics, which conflicts with bit-for-bit integer scoring.
- Rego's nondeterministic builtins and its Datalog-style set semantics are the wrong shape for scoring arithmetic.
- Wasm is opaque to league members (not auditable) and pushes compilers onto clients. It suits server plugins, not admin-authored rules.
- CEL reproducibility "years later" depends on pinning the environment and library version. cel-go makes breaking changes (v0.32 module path, v0.33 cost model default), so stored rules should record an environment version, as Kubernetes does.
- Starlark is a good choice if you need multi-statement procedural rules. Its exact big ints and ordered dicts help determinism, but you need step limits and a lint pass, and a non-Go re-implementation is a bigger job.

### Gaps
- I did not fetch the starlark-rust (Meta) or Kotlin/Swift CEL implementations. Their status is unverified.
- Conformance pass rates for `@bufbuild/cel`, cel-rust and cel-java were not found.
- I found no written backward-compatibility policy for cel-spec or Expr.
- I did not confirm Expr's map iteration determinism, its float/int promotion rules, or any ports.
- I did not get the exact Kubernetes per-expression budget numbers. They are in the k8s source, not the docs page.
- I did not verify wazero fuel and determinism support.
- The JSONLogic community spec's status and official URL are unconfirmed (only a mirror page was found).

## Hybrid patterns (declarative modules plus an expression escape hatch)

### Takeaway
Production systems embed a small expression language inside a declarative, schema-validated config. Kubernetes uses CEL inside CRD and admission-policy fields with typed variables. Google IAM uses a CEL subset inside role bindings. flagd uses JSONLogic with custom deterministic operators inside a flag schema. This maps directly onto "typed scoring modules plus optional CEL expression fields".

### Cited Findings
- Kubernetes puts CEL "one-liners" into string fields of API resources, with declared per-field inputs (`self`, `oldSelf`). It is evaluated in-process as "a convenient alternative to out-of-process mechanisms, such as webhooks". — [Kubernetes CEL docs](https://kubernetes.io/docs/reference/using-api/cel/)
- Kubernetes mixes full and partial type checking. CRD validation rules are fully type checked, while in ValidatingAdmissionPolicy `request` is typed but `object` is dynamic. — [Kubernetes CEL docs](https://kubernetes.io/docs/reference/using-api/cel/)
- Google IAM Conditions use "a subset of the Common Expression Language (CEL)", attached to bindings, with attributes such as `request.time` and `resource.name`. — [Google IAM Conditions overview](https://docs.cloud.google.com/iam/docs/conditions-overview), [Italian version](https://docs.cloud.google.com/iam/docs/conditions-overview?hl=it)
- flagd (OpenFeature) uses "a modified version of JsonLogic" for targeting. It adds custom ops: `fractional` ("Deterministic, pseudorandom fractional distribution"), `sem_ver` and `starts_with`. Custom ops return null on invalid input. Shared `$evaluators` reduce duplication but cannot nest. — [flagd flag definitions](https://flagd.dev/reference/flag-definitions/)
- protovalidate-es runs a native fast path for common rules and falls back to `@bufbuild/cel` for the rest. This is the same "declarative modules plus CEL escape hatch" pattern. — [protovalidate-es](https://openapps.pro/packages/protovalidate-es)

### Inferences
- Suggested shape:
  - Most league settings (bonus/malus values per event, modifiers, thresholds) live as typed declarative modules with integer-hundredths fields. These are easy to render and diff.
  - An optional `expr` field holds a CEL expression, evaluated against a typed, documented activation (for example `player.goals`, `player.vote`, `match.home`).
  - The result type is constrained to `int`.
- flagd's custom-op approach shows how to add domain functions (for example `round_half_up(x, step)`) while keeping them pure and specified in the protocol.

### Gaps
- No primary sources were fetched on insurance or tax rule engines (for example DMN/FEEL, OpenFisca) or on Envoy's CEL usage. The Envoy RBAC/ext_authz CEL usage is known but uncited here.

## Security risks of server-side user expressions and mitigations

### Takeaway
The main risks are resource exhaustion (huge ASTs, exponential macros, regex blowup, deep recursion) and capability leakage through host-exposed functions or struct fields. Mitigations are layered: input size and AST node limits, static cost estimation at write time, runtime cost budgets, a minimal function environment, and pinned library versions.

### Cited Findings
- Expr CVE-2025-29786: a very large expression caused OOM during parsing. The recommended mitigation is node and memory limits, or a maximum expression length before parsing. — [GitLab advisory](https://advisories.gitlab.com/pkg/golang/github.com/expr-lang/expr/CVE-2025-29786/)
- Expr later added fixes that show the attack surface: "Limit recursion depth in built-in functions", "Prevent stack underflow in the VM", and "Disallow access to unexported struct fields". — [expr releases](https://github.com/expr-lang/expr/releases)
- cel-go added parser and checker node limits (v0.30.0), regex program size limits (v0.31.0) and runtime peak-memory tracking (v0.33.0). — [cel-go releases](https://github.com/cel-expr/cel-go/releases)
- In CEL, macros are the only path to exponential behavior. — [cel-spec](https://github.com/google/cel-spec/blob/master/doc/langdef.md)
- Kubernetes estimates regex cost as `len(regex)*len(input)`, enforces runtime budgets, and rejects expressions with too high an estimated cost at write time. — [Kubernetes CEL docs](https://kubernetes.io/docs/reference/using-api/cel/)
- gopher-lua opens all libraries by default unless `SkipOpenLibs` is set. A safe setup must exclude modules with file or system access. — [gopher-lua](https://github.com/yuin/gopher-lua)
- Rego includes `http.send` and DNS lookups, which are SSRF and nondeterminism vectors if not disabled. — [OPA builtins](https://www.openpolicyagent.org/docs/policy-reference/builtins)

### Inferences
- Validate at write time: parse, type-check, compute the estimated cost against a fixed cap, and reject on failure.
- At run time: set a cost limit and a context timeout, and treat any error as a deterministic "rule error" outcome recorded in results, so the outcome is still reproducible.
- Expose only plain data (protobuf or declared types), never Go structs with methods.

### Gaps
- No public security audit report for cel-go or Expr was found.

## Auditability and human-readable rendering for non-technical league members

### Takeaway
I found little direct sourcing on rendering rules for humans. The best available levers are CEL's checked AST (typed, so it can be translated into natural language) and storing declarative modules as structured data that diffs field by field.

### Cited Findings
- CEL expressions are short one-liners stored in resource string fields. Kubernetes records them with declared variable names, which makes them readable in YAML diffs. — [Kubernetes CEL docs](https://kubernetes.io/docs/reference/using-api/cel/)
- JSONLogic rules are plain JSON data, so standard JSON diff tools apply. — [jsonlogic.com](https://jsonlogic.com/)
- `@bufbuild/cel-spec` exposes parsed ASTs and inferred types (sourced from cel-go) for tooling. — [@bufbuild/cel-spec README](https://cdn.jsdelivr.net/npm/@bufbuild/cel-spec@0.4.0/README.md)

### Inferences
- Render declarative modules from a localized template (for example "Goal by a defender: +4.50"). Diff them semantically, field by field.
- For CEL escape hatches:
  - Store the source text and an environment version.
  - Show a canonical pretty-print, produced by unparsing the AST.
  - Optionally show an auto-generated Italian/English gloss produced by walking the checked AST.
  - Require a short admin-written description.
- Provide "explain" output during scoring: per-player evaluation traces showing which modules and expressions contributed which hundredths. CEL's deterministic evaluation makes such traces reproducible.

### Gaps
- No sources were found on products that auto-generate natural-language explanations from CEL or JSONLogic ASTs.
- I did not research cel-go's unparser API (`parser.Unparse`); its existence is from memory, not cited.
