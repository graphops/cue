Title: evaluator: try clause drops what it yields into a field its own expression refers to

No existing issue covers this (searched again on 2026-11-01). #4485 and #4486
are different `try` shapes; see Analysis.

### What version of CUE are you using (`cue version`)?

v0.18.0-rc.3, and master at a0768bd98, which is the same tree.

### Does this issue reproduce with the latest stable release?

Yes, v0.17.1 fails in the same way. v0.16.1 and v0.17.0-alpha.1 give the
expected output.

### What did you do?

```
! exec cue export drop.cue
cmp stderr drop.err

exec cue export cyc.cue
cmp stdout cyc.json

-- drop.cue --
@experiment(try)

x: a: 5
try X = x? {x: a: <=2}
-- drop.err --
x.a: invalid value 5 (out of bound <=2):
    ./drop.cue:4:19
    ./drop.cue:3:7
-- cyc.cue --
@experiment(try)

x: a: 5
try X = x? {x: b: 1}
-- cyc.json --
{
    "x": {
        "a": 5,
        "b": 1
    }
}
```

### What did you expect to see?

`x` exists, so both try clauses yield their body. `drop.cue` fails because
`x.a` is 5, which is out of bound `<=2`, and `cyc.cue` exports
`{"x": {"a": 5, "b": 1}}`. v0.16.1 does both.

### What did you see instead?

`drop.cue` exports without an error, and the constraint is lost:

```
{
    "x": {
        "a": 5
    }
}
```

`cyc.cue` fails:

```
x: cyclic reference to field b:
    ./cyc.cue:4:1
    ./cyc.cue:3:1
```

With a dynamic field, as in `try X = x? {x: ("b"): 1}`, the field is lost
without an error as well.

### Analysis

Bisected to 339485ddf ("internal/core/adt: dependency-tracking comprehension
pushdown", https://cuelang.org/cl/1236923, first in v0.17.0-alpha.2). Its
parent passes.

`TryClause.yield` (expr.go) finalizes an inline vertex for the expression `x?`
to find out whether a `?`-marked reference fails. That vertex shares `x`, so
`Vertex.unify` and `nodeContext.finalizeSharing` (share.go) finalize `x`, and
`completeAllArcs` finalizes `x.a`/`x.b`. Since 339485ddf, `pushDownDeps`
(comprehension.go) makes the try comprehension a parent task of these arcs,
and that comprehension has not yielded yet. When it does,
`Vertex.insertConjunct` (fields.go) finds `x` finalized and drops the body's
conjunct without an error. In `cyc.cue`, `x.b` is still a pending placeholder
when `x` is completed, and `updateArcType` (composite.go) reports the cycle.
For the dynamic field, the pre-evaluation also freezes the field set of `x`:
`scheduler.deferFieldSetKnown` (sched.go) does not defer the freeze for the
current task, which is the try comprehension. I confirmed each step with
debug prints.

This is one instance of a pattern from 339485ddf: a field is finalized while a
pushed-down comprehension that yields into it is still running. Here the field
is finalized by the comprehension's own try clause.

`if` and `for` clauses do not finalize what they read (they evaluate with
`yield` or `attemptOnly`), so `if x.a > 0 {x: a: <=2}` works. When a guard
does need the final value, as `if x != _|_ {x: b: 1}` does, the cycle is
reported, and `cycle/compbottom2.txtar` relies on that. The try clause is
different: its decision only depends on whether its `?`-marked references
resolve, and finalizing what they refer to is an implementation detail of
the pre-evaluation.

Related: #4485 (closed, fixed by 5e2823acf) is `try { x: b: x.a? }`, a body
reading a field it also sets. It is fixed on master and is not this bug. #4486
(open) is a nested `try` whose target lookup freezes the type of a
placeholder arc. That is a different path, and the fix proposed below does not
fix #4486 or interact with a fix for it.

### Proposed approach

While a comprehension pre-evaluates a try clause, mark its task. While a
parent task of a vertex is marked:

- `Vertex.unify` lowers a request to finalize the vertex to `attemptOnly`. A
  later finalize completes the vertex once the comprehension has yielded. This
  one check covers every path the pre-evaluation takes to these arcs.
- `deferFieldSetKnown` treats the marked task like any other parent task, so
  it does not freeze the field set of the vertex.

The exemption covers only the pre-evaluation, not the rest of the
comprehension. An `if` clause after the `try`, as in
`try Y = y? if x != _|_ {x: b: 1}`, then still reports the same cycle as
without the `try`. Marking every running comprehension that has a try clause
would hide that cycle. Marking every running comprehension would also hide
the cycles in `cycle/compbottom2.txtar`.

I have a change with a regression test in `cue/testdata/comprehensions/try.txtar`
ready. No existing test output or stats change.
