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

## Additions

Not bug fixes: features configurations built on this fork rely on. They exist
only on this branch.

| Addition | What it adds | Test |
|---|---|---|
| `struct.Data(v)` | `v` as concrete data (regular fields, defaults resolved, no closedness, definitions, hidden or optional fields), built as fresh CUE literals. An in-CUE replacement for `json.Unmarshal(json.Marshal(v))`, about 2.7× cheaper, which keeps CUE kinds such as bytes. | `pkg/struct/testdata/data.txtar` |
| Sharing past a repeated definition | Structure sharing is kept when a conjunct refers to a definition the shared value already applied (plainly, or opened for an opened repeat); the repeated conjunct is remembered and applied as usual if the node is unshared. A schema-checked value can then be passed to many places that also declare the schema. | `cue/testdata/eval/sharedefs.txtar` |

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
