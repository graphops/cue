# cue: values built by Context.BuildExpr skip closedness checks, so `cue export -e '#S & d'` accepts fields #S does not allow

## Reproducer

```txtar
# In a file, #S & d is rejected: #S does not allow b.
! exec cue export x.cue
cmp stderr want-file

# The same conjunction given with -e must be rejected too.
! exec cue export -e '#S & d' x.cue
cmp stderr want-expr
! exec cue eval -e '#S & d' x.cue
cmp stderr want-expr

-- x.cue --
#S: {a?: int}
d: {b: 1}
x: #S & d
-- want-file --
x.b: field not allowed:
    ./x.cue:2:5
-- want-expr --
b: field not allowed:
    ./x.cue:2:5
```

### Expected

All three commands fail. The two `-e` commands report `b: field not allowed`, as the
same conjunction does when it is written in the file.

### Actual

On v0.18.0-rc.3 (= master), the file is rejected, but the `-e` commands succeed:

```
> ! exec cue export -e '#S & d' x.cue
[stdout]
{
    "b": 1
}
FAIL: repro.txtar:6: unexpected command success
```

`cue eval -e '#S & d'` prints `b: 1`, and `cue def -e '#S & d'` prints the expression
without error. Nested forms such as `-e '{y: #S & d}'`, `-e '[#S & d][0]'` and
`-e '#S & {b: 1}'` are accepted too. The Go API has the same problem:
`ctx.BuildExpr(expr, cue.Scope(v)).Validate()` returns nil. `cue vet -d '#S & d'` is not
affected.

## Versions

- v0.18.0-rc.3 / master: fails
- v0.18.0-rc.2: fails
- v0.18.0-rc.1: fails
- v0.17.1: fails
- v0.16.1: passes
- Bisected to a9f3f2a09 ("internal/core/adt: skip typo check on cross-context Finalize",
  v0.17.0-alpha.2, fix for #4366): its parent passes and it fails.

## Root cause

`Context.BuildExpr` (cue/context.go) evaluates the expression with the OpContext `c.ctx()`
through `adt.Resolve` (internal/core/adt/adt.go). A conjunction is an `Evaluator`, so
`Resolve` calls `OpContext.Evaluate`. That returns a vertex whose `status` is still
`evaluating`, with `v.state` owned by that OpContext. `BuildExpr` then passes the vertex to
`Context.make`. `make` creates a new OpContext, and `newVertexRoot` (cue/types.go) calls
`x.Finalize` with it. In `Vertex.unify` (internal/core/adt/unify.go),
`v.state.ctx.opID != c.opID` now holds, so it records a `GenerationMismatch` and, since
a9f3f2a09, sets `checkTypos = false`. That change assumes the original evaluation will run
the typo check when its own Finalize unwinds. `BuildExpr` never finalizes in the original
context, so nothing runs the check. A debug build of rc.3 confirms this for
`-e '#S & d'`: the vertex leaves `Resolve` with status `evaluating`, and the next Finalize
hits the mismatch with `checkTypos` true on entry.

## Proposed fix

Call `v.Finalize(ctx)` in `BuildExpr` after `adt.Resolve`, with the same OpContext, before
`c.make(v)`. `Context.Encode` already does this. The vertex is then `finalized` before the
second OpContext sees it, so the typo check runs with consistent defIDs, and the Finalize in
`make` does nothing. The change is 5 lines in cue/context.go, plus `TestBuildExprClosedness`
in cue/build_test.go and cmd/cue/cmd/testdata/script/eval_e_closedness.txtar. It leaves the
a9f3f2a09 skip in place for the case it targets: re-entrant `value.Make` during evaluation,
as in #4366. Limit: the fix only covers `BuildExpr`. Other API paths that return a partly
evaluated vertex through a new OpContext may have the same gap; we did not audit them. Fork
commit: graphops/cue branch `fix/rc3-buildexpr-closedness`, 8681680.

## Related

- #4366 (closed): the issue whose fix, a9f3f2a09, introduced the typo-check skip.
- #4349 (closed): `-e` with `cue def --out jsonschema` loses nested closedness. That is a
  different mechanism (jsonschema encoding), not a duplicate.
- No existing issue found for this bug. Searched for "eval -e field not allowed",
  "export -e field not allowed", "BuildExpr closedness", "BuildExpr closed",
  "GenerationMismatch" and "cross-context Finalize", and listed issues matching
  "closedness -e" and "eval -e not allowed".
