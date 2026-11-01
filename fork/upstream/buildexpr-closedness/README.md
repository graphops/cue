# cue: values built by Context.BuildExpr skip closedness checks, so `cue eval -e '#S & d'` accepts fields #S does not allow

### What version of CUE are you using (`cue version`)?

<pre>
$ cue version
cue version v0.18.0-rc.3
</pre>

Also reproduced with master at a0768bd98.

### Does this issue reproduce with the latest stable release?

Yes, with v0.17.1. It does not reproduce with v0.16.1.

### What did you do?

```
# In a file, #S & d is rejected: #S does not allow b.
! exec cue export x.cue
cmp stderr want-file

# The same conjunction given with -e must be rejected too.
! exec cue export -e '#S & d' x.cue
cmp stderr want-expr
! exec cue eval -e '#S & d' x.cue
cmp stderr want-expr
! exec cue eval -e '{y: #S & d}' x.cue
cmp stderr want-nested

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
-- want-nested --
y.b: field not allowed:
    ./x.cue:2:5
```

### What did you expect to see?

The testscript passes: every command fails with `field not allowed`, as the
conjunction does when it is written in the file, and as it did with v0.16.1.

### What did you see instead?

The file is rejected, but the expressions given with `-e` are accepted:

```
> ! exec cue export -e '#S & d' x.cue
[stdout]
{
    "b": 1
}
FAIL: repro.txtar:6: unexpected command success
```

`cue eval -e '#S & d'` prints `b: 1`, `cue eval -e '{y: #S & d}'` prints
`y: b: 1`, and `cue def -e '#S & d'` succeeds too. The Go API behaves the same:
`ctx.BuildExpr(expr, cue.Scope(v)).Validate()` returns nil for `#S & d`.
`cue vet -d '#S & d'` is not affected.

### Analysis

Bisected to a9f3f2a09 ("internal/core/adt: skip typo check on cross-context
Finalize", the fix for #4366, first in v0.17.0-alpha.2); its parent passes.
Since then `Vertex.unify` skips the typo check when an OpContext other than
the one which started evaluating a vertex finalizes it, as the original
evaluation is expected to run the check when its own `Finalize` unwinds.

`Context.BuildExpr` (cue/context.go) never does that. It evaluates the
expression with `adt.Resolve`, which for a conjunction or a struct literal
calls `OpContext.Evaluate` and returns a vertex still `evaluating`, its arcs
unprocessed. It then passes the vertex to `Context.make`, whose
`newVertexRoot` (cue/types.go) finalizes it with a new OpContext: the
`GenerationMismatch` branch fires and the typo check is skipped, so nothing
ever runs it. A debug build confirms this for `-e '#S & d'`.

The other `Context` and `Value` methods we checked (`CompileString`,
`BuildFile`, `Encode`, `NewList`, `LookupPath`, `FillPath` with an
`ast.Expr`, `Unify`) are not affected: they either hand `make` a vertex no
context has evaluated yet, or finalize their result in the context that
evaluated it.

This is unrelated to the comprehension pushdown (339485ddf) behind several
other recent regressions.

It also undoes, for `-e`, the closing that #2680 asked for:
`cue eval -e 'close({}) & w'` with `w: a: 1` prints `a: 1` with v0.17.1 and
rc.3, while v0.16.1 and the proposed change report `a: field not allowed`. The fix for #2680
(2c281cb56) is a different mechanism, and its test, which unifies the built
value later with `Value.Unify`, still passes.

### Proposed approach

Finalize the vertex in `BuildExpr` with the OpContext that evaluated it,
before `c.make(v)`, as `Context.Encode` already does. The vertex is then
finalized with consistent closedness IDs, the `Finalize` in `make` does
nothing, and the skip from a9f3f2a09 stays in place for the re-entrant case
it targets (#4366). The change is one line in cue/context.go plus a case in
cmd/cue/cmd/testdata/script/eval_e.txtar; all tests pass with it.

An alternative is to finalize inside `adt.Resolve`, where a
`// TODO: Finalize here?` already sits. That also passes all tests, but it
changes the other two callers (`Value.FillPath` with an `ast.Expr` and cue
struct tags in internal/core/convert), which are not affected, so we prefer
the local change. Fixing it in `make` or `newVertexRoot` is not possible:
they do not know the original context, and in the #4366 case that context
is still mid-evaluation.

We have a commit ready following this approach and can send it once the
approach is agreed.
