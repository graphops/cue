# GraphOps fork of CUE v0.18.0-rc.1

Evaluator fixes and additions GraphOps needs to run its configurations on CUE
v0.18.0-rc.1, until equivalent changes land upstream. Nothing here has been
reported or proposed upstream yet; each fix is meant to become an upstream
issue and change once its analysis is reduced to a self-contained case.

This branch starts at the `v0.18.0-rc.1` tag. See `GRAPHOPS.md` on `main` for
the fork's branch conventions.

## Fixes

Each fix also exists on its own branch, isolated on the `v0.18.0-rc.1` tag.

| Branch | What it fixes | Confidence |
|---|---|---|
| `fix/rc1-uninit-parent-state` | Panic "vertex … not initialized": a bare parent state, from a reclaimed and revived vertex | Medium-high |
| `fix/rc1-lookup-settle-arc` | Incomplete `len()` frozen by a lookup forcing unrelated tasks | Medium-high |
| `fix/rc1-let-pushdown` | Silent value loss: a `let` in a comprehension body blocks its pushdown | High |
| `fix/rc1-try-finalize` | Silent constraint loss or spurious cycle: a try clause's target arcs finalized before it yields | High |
| `fix/rc1-ancestor-flags` | Ancestor completion flags attributed while parent tasks are pending | Hardening, no known impact |
| `fix/rc1-try-target-probe` | Spurious cycle from a try nested in another try's body | High |
| `fix/rc1-ready-parent-completion` | Non-concrete value: a pushed-down arc completes before its comprehension has run | High |
| `fix/rc1-else-pushdown` | Same, for an arc only an `else` clause declares | High |
| `fix/rc1-running-producer` | Silent constraint loss: a try body discarded because the arc type it reads was frozen while its producer comprehension was running | Medium-high |
| `fix/rc1-call-memo-copies` | Performance: every copy of a function result re-ran its nested calls | Medium-high |
| `fix/rc1-call-arg-cycle` | Spurious structural cycle: a call inherited from a cycle its arguments do not reach was postponed until it became a structural cycle | Medium-high |
| `fix/rc1-call-memo-bindings` | Performance: calls on comprehension variables re-ran in every copy; also a wrong result when a pattern passes its label alias to a function. Based on `fix/rc1-call-memo-copies` | Medium |
| `fix/rc1-pushdown-notify` | Silent value loss: a reference copied a struct while a pushed-down comprehension targeting it was still running, so the fields the comprehension added afterwards never reached the referring node | Medium-high |
| `fix/rc1-arctype-validate` | Incomplete value, order-dependent: iterating a struct before it is evaluated validated a comprehension-declared arc while its reference was unresolved, fixing it to `_` ("undefined as v is incomplete (type _)"; the arc's fields lost) | Medium-high |
| `fix/rc1-export-let` | Panic "unreachable" (or a stack overflow, or a silently detached expression) printing non-concrete values: a `let X = self` referenced outside the struct declaring it, as through `#T: #S & {}`, was expanded in place, and comprehension clause expressions were exported one environment too deep. Affects `cue eval` and `Value.Syntax`, not evaluation or `cue export` | Medium-high |
| `fix/rc1-buildexpr-closedness` | Silent constraint loss: a conjunction built with `Context.BuildExpr`, as by `cue eval -e '#S & d'`, skipped definition closedness, because its value was finalized in a second evaluation context that suppresses typo checks | High |
| `fix/rc1-nested-disjunction-errors` | Lost diagnostics: when every disjunct failed only through a nested disjunction (e.g. each reads a field whose own disjunction failed), the error was a bare "empty disjunction" without any cause | High |
| `fix/rc1-fmt-alias-form` | `cue fmt` wrote the single postfix alias as `~(X)` for files whose language version predates it (below v0.17.0, which cannot parse it); it now keeps `~X` there | High |
| `fix/rc1-struct-data-incomplete` | `struct.Data` (an addition below) turned an argument that was only incomplete, such as one reading an optional field, into a permanent error, failing evaluation of a field nobody asked to be concrete; it now keeps the argument's error code, as `json.Unmarshal(json.Marshal(v))` does. Based on the `struct.Data` addition | High |
| `fix/rc1-share-funcarg-cycle` | Spurious structural cycle: with the sharing addition below, a function applied to the result of an earlier call of the same call site, with a definition on its argument, reported a structural cycle. The addition no longer keeps a node shared that shares a function argument. Based on the sharing addition; the same cycle without a definition is an upstream issue this does not fix | Medium-high |

## Additions

Not bug fixes: features configurations built on this fork rely on. They exist
only on this branch.

| Addition | What it adds | Test |
|---|---|---|
| `struct.Data(v)` | `v` as concrete data (regular fields, defaults resolved, no closedness, definitions, hidden or optional fields), built as fresh CUE literals. An in-CUE replacement for `json.Unmarshal(json.Marshal(v))`, about 2.7× cheaper, which keeps CUE kinds such as bytes. An incomplete argument makes the call incomplete, as with the JSON round trip. | `pkg/struct/testdata/data.txtar`, `cmd/cue/cmd/testdata/script/struct_data_incomplete.txtar` |
| Sharing past a repeated definition | Structure sharing is kept when a conjunct refers to a definition the shared value already applied (plainly, or opened for an opened repeat); the repeated conjunct is remembered and applied as usual if the node is unshared. A schema-checked value can then be passed to many places that also declare the schema. Not applied to a node sharing a function argument. | `cue/testdata/eval/sharedefs.txtar`, `cue/testdata/functions/sharedefs_reapply.txtar` |

## Releases

Pushing a tag `v0.18.0-rc.1-graphops.<n>` on this branch runs
`.github/workflows/release.yml`: it tests the fork and publishes a GitHub
release with `cue` binaries for Linux, macOS and Windows (amd64 and arm64),
named like upstream's (`cue_<tag>_<os>_<arch>.tar.gz`), plus `sha256sums.txt`.
`cue version` reports the tag. Increment `<n>` for each release from this
branch; a manual run of the workflow builds a snapshot without publishing.

Go programs that embed CUE use a release with a replace directive:

```
require cuelang.org/go v0.18.0-rc.1
replace cuelang.org/go => github.com/graphops/cue v0.18.0-rc.1-graphops.1
```

## Build and test

```sh
git checkout graphops/v0.18.0-rc.1
go build -o cue ./cmd/cue
go test ./...
```

Every fix passes `go test ./...`, has a regression test that fails without
it, and names the upstream commit that introduced the bug where bisection
found one. The detailed analyses are kept privately, because their original
reproducers come from private configuration.
