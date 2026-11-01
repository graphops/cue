# GraphOps fork of CUE v0.18.0-rc.3

Evaluator fixes and additions GraphOps needs to run its configurations on CUE
v0.18.0-rc.3, until equivalent changes land upstream. Nothing here has been
reported or proposed upstream yet; each fix is meant to become an upstream
issue and change once its analysis is reduced to a self-contained case.

This branch starts at the `v0.18.0-rc.3` tag. See `GRAPHOPS.md` on `main` for
the fork's branch conventions. It carries over every fix and addition of
`graphops/v0.18.0-rc.2` that upstream has not made unnecessary; see
[Moving from v0.18.0-rc.2](#moving-from-v0180-rc2).

## Fixes

Each fix also exists on its own branch, isolated on the `v0.18.0-rc.3` tag.

| Branch | What it fixes | Confidence |
|---|---|---|
| `fix/rc3-uninit-parent-state` | Panic "vertex … not initialized": a bare parent state, from a reclaimed and revived vertex | Medium-high |
| `fix/rc3-let-pushdown` | Silent value loss: a `let` in a comprehension body blocks its pushdown | High |
| `fix/rc3-try-finalize` | Silent constraint loss or spurious cycle: a try clause pre-evaluates its expression by finalizing an inline vertex, which may finalize the fields its body yields into before it yields (`try X = x? {x: a: <=2}` exported `x.a: 5`; `{x: b: 1}` was a cyclic reference; a dynamic field was dropped). While the clause pre-evaluates, finalizing a vertex it is a parent task of is lowered to attemptOnly, and the field set is not frozen; other clauses still report their cycles. Since `339485ddf` (v0.17.0-alpha.2) | High |
| `fix/rc3-ancestor-flags` | Ancestor completion flags attributed while parent tasks are pending | Hardening, no known impact |
| `fix/rc3-try-target-probe` | Spurious cycle from a try nested in another try's body | High |
| `fix/rc3-ready-parent-completion` | Non-concrete value: a pushed-down arc completes before its comprehension has run | High |
| `fix/rc3-running-producer` | Silent constraint loss: a try body discarded because the arc type it reads was frozen while its producer comprehension was running | Medium-high |
| `fix/rc3-call-memo-copies` | Performance: every copy of a function result re-ran its nested calls | Medium-high |
| `fix/rc3-call-arg-cycle` | Spurious structural cycle: a call inherited from a cycle its arguments do not reach was postponed until it became a structural cycle | Medium-high |
| `fix/rc3-call-memo-bindings` | Performance: calls on comprehension variables re-ran in every copy; also a wrong result when a pattern passes its label alias to a function. Based on `fix/rc3-call-memo-copies` | Medium |
| `fix/rc3-pushdown-notify` | Silent value loss: a reference copied a struct while a pushed-down comprehension targeting it was still running, so the fields the comprehension added afterwards never reached the referring node | Medium-high |
| `fix/rc3-arctype-validate` | Incomplete value, order-dependent: iterating a struct before it is evaluated validated a comprehension-declared arc while its reference was unresolved, fixing it to `_` ("undefined as v is incomplete (type _)"; the arc's fields lost) | Medium-high |
| `fix/rc3-export-let` | Panic "unreachable" (or a stack overflow, or a silently detached expression) printing non-concrete values: a `let X = self` referenced outside the struct declaring it, as through `#T: #S & {}`, was expanded in place, and comprehension clause expressions were exported one environment too deep. Affects `cue eval` and `Value.Syntax`, not evaluation or `cue export` | Medium-high |
| `fix/rc3-buildexpr-closedness` | Silent constraint loss: a conjunction built with `Context.BuildExpr`, as by `cue eval -e '#S & d'`, skipped definition closedness, because its value was finalized in a second evaluation context that suppresses typo checks | High |
| `fix/rc3-nested-disjunction-errors` | Lost diagnostics: when every disjunct failed only through a nested disjunction (e.g. each reads a field whose own disjunction failed), the error was a bare "empty disjunction" without any cause | High |
| `fix/rc3-fmt-alias-form` | `cue fmt` wrote the single postfix alias as `~(X)` for files whose language version predates it (below v0.17.0, which cannot parse it); it now keeps `~X` there | High |
| `fix/rc3-struct-data-incomplete` | `struct.Data` (an addition below) turned an argument that was only incomplete, such as one reading an optional field, into a permanent error, failing evaluation of a field nobody asked to be concrete; it now keeps the argument's error code, as `json.Unmarshal(json.Marshal(v))` does. Based on the `struct.Data` addition | High |
| `fix/rc3-share-disjunction-safe` | Spurious "cyclic reference to field" or lost fields, a regression in v0.18.0-rc.2: upstream `082c52c57` finalizes the vertex a field shares when the field meets a disjunction, to keep sharing. When the field is only evaluated in part, from within an evaluation the vertex depends on (a comprehension of the vertex, or of a vertex it refers to, reading the field), that evaluation was forced before it added its fields. The vertex is now finalized early only while the sharing field is itself being finalized. The self-reference half of the rc.2 fix is upstream since `4232b8d36` | High |
| `fix/rc3-share-funcarg-chain` | Spurious structural cycle, upstream since functions exist: a function applied, through a wrapper, to the result of an earlier call of the same call site, without any definition. A reference to a parameter that structure shares its argument was dereferenced to the argument's value, evaluated with the callee body's cycle-reference chain instead of the caller's; it now stops at the parameter, whose argument conjunct carries the caller's chain. With the sharing addition below, it also covers the same cycle with a definition on the argument (test `cue/testdata/functions/sharedefs_reapply.txtar`, carried from the former `fix/rc2-share-funcarg-cycle`) | Medium-high |
| `fix/rc3-comprehension-error-loss` | Silent value loss: a field declared next to a struct-level comprehension (such as `x: [...]`) kept its declared value when the comprehension's clauses failed, because the error was recorded only on the enclosing struct; a reference evaluated before the struct exported `[]` with exit 0. The error is now added as a conjunct of each field the body or else clause declares, so embeddings and loops over the field see it too. Incomplete and else cases since `339485ddf` (v0.17.0-alpha.2); a fatal guard without else since `748a68589` (v0.5.0-alpha.1) | High |
| `fix/rc3-call-closedness-path` | Lost diagnostics, a regression in v0.18.0-rc.2: upstream `7bf42f62f` reports an error arising in a function call at the field holding the call, but dropped the error's path within the argument, so a closedness error below an argument read only "field not allowed", and distinct failures at the same call were deduplicated into one. An error below an argument is now wrapped in an error naming its path from the parameter (`w: cfg.extra: field not allowed`), whatever its kind, also when it wraps another error such as a failed validator | High |
| `fix/rc3-deferred-disjunct-scope` | Spurious "field not allowed" and silently accepted invalid values, a regression in v0.18.0-rc.2: upstream `b68cd7163` completes a field of an inline struct or let that a task read too early with a task recorded by `rootedTaskNode`, which searched the whole task stack. Inside a disjunct, that recorded the completion outside of it: an error of a discarded disjunct failed the referrer, and a let's error landed in another disjunct, accepting an invalid one. The search now only considers the tasks the innermost disjunct started (the task stack depth recorded in its overlay frame) | High |

## Additions

Not bug fixes: features configurations built on this fork rely on. They exist
only on this branch and, with the fix based on each, on the fix branches noted
above.

| Addition | What it adds | Test |
|---|---|---|
| `struct.Data(v)` | `v` as concrete data (regular fields, defaults resolved, no closedness, definitions, hidden or optional fields), built as fresh CUE literals. An in-CUE replacement for `json.Unmarshal(json.Marshal(v))`, about 2.7× cheaper, which keeps CUE kinds such as bytes. An incomplete argument makes the call incomplete, as with the JSON round trip. | `pkg/struct/testdata/data.txtar`, `cmd/cue/cmd/testdata/script/struct_data_incomplete.txtar` |
| Sharing past a repeated definition | Structure sharing is kept when a conjunct refers to a definition the shared value already applied (plainly, or opened for an opened repeat); the repeated conjunct is held and applied as usual if the node is unshared. A schema-checked value can then be passed to many places that also declare the schema. Upstream keeps sharing since `1c287ec21` when the shared value holds the definition as a direct conjunct; this extends it to definitions applied any other way (such as through a field of an opened definition), to a definition arriving before the value, and to opened definitions. Not applied to a plain definition re-closing an opened reference (`x... & #D`). | `cue/testdata/eval/sharedefs.txtar`, `cue/testdata/functions/sharedefs_reapply.txtar` |

## Moving from v0.18.0-rc.2

Every fix of `graphops/v0.18.0-rc.2` was checked on plain v0.18.0-rc.3 by
applying only its tests, then reapplied where its regression test still
fails. Plain v0.18.0-rc.3 passes `go test ./...`.

Dropped, because v0.18.0-rc.3 fixes them upstream:

- `fix/rc2-else-pushdown`: upstream `154d2ce09` makes the same change
  ("push down the else clause of a comprehension").
- `fix/rc2-lookup-settle-arc`: its test passes on plain v0.18.0-rc.3.
  Upstream `e147bfc48` stops lookups from forcing pushed-down comprehensions
  at all, which replaces the rc.2 shortcut this fix corrected.
- Half of `fix/rc2-share-disjunction-safe`: re-sharing what the finalized
  field shares, which kept a definition from being dropped as a
  self-reference. Upstream `4232b8d36` compares a recorded reference arc by
  identity, which fixes that at its source; the re-sharing also skipped the
  links upstream's new `sharesOpened` (`55ea1639a`) reads, closing a node
  that an opened field in the chain had opened (`explicitopen.txtar`,
  `forwardShared`). Its tests are kept and pass.

Not carried, as on `graphops/v0.18.0-rc.2`: `fix/rc2-share-disjunction-finalize`
(a revert of `082c52c57` that rc.2-graphops.2 reverted again) and the code of
`fix/rc2-share-funcarg-cycle` (subsumed by `fix/rc2-share-funcarg-chain`; its
test is carried as its own commit).

Every other fix and both additions are carried. Their regression tests fail on
plain v0.18.0-rc.3 without them. These needed adapting:

- `fix/rc3-let-pushdown`: the code is unchanged. Upstream `e147bfc48` lets a
  lookup decide that a field missing from a node whose only remaining tasks
  are pushed-down comprehensions cannot exist. With the fix, a comprehension
  whose body holds lets is pushed down, so `letCycleSelfKeyword2` in
  `pushdown.txtar` reports "undefined field: hosts" instead of "cannot range
  over s.hosts (incomplete)", still as an incomplete error. Upstream made the
  same change to the `let this = self` case in `nested2.txtar`.
- `fix/rc3-call-arg-cycle`: upstream `897d511cf` moved the call check ahead of
  the deferral in `conjunct.go`; the deferral now tests whether the call's
  arguments reach the cycle within upstream's structure.
- `fix/rc3-share-disjunction-safe`: reduced to its in-progress check
  (since replaced, see below).
- `fix/rc3-share-funcarg-chain`: placed next to upstream's new `sharesOpened`;
  the code is unchanged.
- `fix/rc3-export-let`, `fix/rc3-call-memo-copies`,
  `fix/rc3-call-memo-bindings`, `fix/rc3-call-closedness-path`,
  `fix/rc3-comprehension-error-loss`: golden files only (v0.18.0-rc.3 numbers
  new let names sequentially, reports some errors at a more precise path, and
  moved evaluation counters).

On this branch, the goldens that depend on several fixes together are
updated in one commit.

## Revised while preparing upstream changes

Batch 1 of the upstream preparation re-derived eight fixes from first
principles, as single commits on `cjorge-graphops/cue` branches based on
upstream master, with issue drafts on this repository's branch
`upstream/rc3-batch1` (`fork/upstream/`). Where that produced a better fix,
it replaced the one here, as commits on top of this branch and of the
`fix/rc3-` branch; render parity with `v0.18.0-rc.3-graphops.1` holds:

- `fix/rc3-share-disjunction-safe`: the walk for an evaluation in
  progress missed one in a vertex the shared one refers to (`x: y`); now
  a condition on the sharing field (`fix/share-disjunction-in-progress`).
- `fix/rc3-call-closedness-path`: the same output with a third of the
  code and no new error type (`fix/call-error-argument-path`).
- `fix/rc3-try-finalize`: the exemption covered every clause of a
  comprehension with a try clause, hiding the cycle of a later if clause,
  and a dynamic field was still rejected; now limited to the try clause's
  pre-evaluation (`fix/try-own-arcs-finalize`).
- `fix/rc3-deferred-disjunct-scope`: the same results without an
  unreachable fallback that loses a let's error when forced
  (`fix/deferred-completion-disjunct-scope`).
- `fix/rc3-comprehension-error-loss`: the error recorded in the fields'
  state was missed by an embedding of the field; now added as a conjunct
  (`fix/comprehension-error-fields`).

`let-pushdown`, `pushdown-notify` and `buildexpr-closedness` kept their
code; their upstream versions only change comments and tests.

## Releases

Pushing a tag `v0.18.0-rc.3-graphops.<n>` on this branch runs
`.github/workflows/release.yml`: it tests the fork and publishes a GitHub
release with `cue` binaries for Linux, macOS and Windows (amd64 and arm64),
named like upstream's (`cue_<tag>_<os>_<arch>.tar.gz`), plus `sha256sums.txt`.
`cue version` reports the tag. Increment `<n>` for each release from this
branch; a manual run of the workflow builds a snapshot without publishing.

Go programs that embed CUE use a release with a replace directive:

```
require cuelang.org/go v0.18.0-rc.3
replace cuelang.org/go => github.com/graphops/cue v0.18.0-rc.3-graphops.1
```

## Build and test

```sh
git checkout graphops/v0.18.0-rc.3
go build -o cue ./cmd/cue
go test ./...
```

Every fix passes `go test ./...`, has a regression test that fails without
it, and names the upstream commit that introduced the bug where bisection
found one. The detailed analyses are kept privately, because their original
reproducers come from private configuration.
