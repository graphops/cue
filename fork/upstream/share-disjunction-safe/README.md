Title: evaluator: "cyclic reference to field" when a comprehension reads a field unified with its struct and a disjunction (v0.18.0-rc.2 regression)

### What version of CUE are you using (`cue version`)?

<pre>
$ cue version
cue version v0.18.0-rc.3

CUE language version v0.18.0
</pre>

Also reproduced with the current master.

### Does this issue reproduce with the latest stable release?

No. v0.17.1 and v0.18.0-rc.1 give the expected result; it starts with v0.18.0-rc.2.

Bisected to https://github.com/cue-lang/cue/commit/082c52c57532, "internal/core/adt: finalize a shared forward reference before a disjunction".

### What did you do?

```
exec cue export x.cue
cmp stdout want

-- x.cue --
#S: {...}
#A: #S | int
x: {
	a: 1
	if out.a == 1 {b: 2}
}
out: x & #A
-- want --
{
    "x": {
        "a": 1,
        "b": 2
    },
    "out": {
        "a": 1,
        "b": 2
    }
}
```

### What did you expect to see?

A passing test. `out` is `x` unified with `{...} | int`, so the guard holds and both `x` and `out` are `{a: 1, b: 2}`. That is what v0.17.1 and v0.18.0-rc.1 print.

### What did you see instead?

```
x: cyclic reference to field b:
    ./x.cue:7:10
    ./x.cue:3:1
```

### Analysis

The comprehension of `x` selects `out.a`, which evaluates `out` in part. `out` shares `x`, and when it meets the disjunction, `sharesSingleValue` finalizes the shared vertex `x` (since 082c52c57) to know whether it can keep sharing. The comprehension that is running is then frozen before it adds `b`: the stack is `IfClause.yield -> ... -> processResolver -> scheduleConjunct -> sharesSingleValue -> Vertex.Finalize -> completeAllArcs -> updateArcType -> reportFieldCycleError`.

The comprehension need not belong to `x` itself: with `y: {a: 1, if out.a == 1 {b: 2}}` and `x: y`, `x` and its fields are not being evaluated, yet finalizing `x` reaches `y` and fails the same way. With an embedded expression in place of the comprehension, as in `x: {a: 1, [if out.a == 1 {b: 2}, {}][0]}`, `cue eval` succeeds but `cue export` panics with "not properly initialized".

### Proposed approach

Finalize the shared vertex early only while the sharing field is itself being finalized (its innermost `unify` call is in `finalize` mode), which is when `finalizeSharing` would finalize the vertex anyway. A field evaluated only in part, as for `out.a` here, keeps the behaviour from before 082c52c57: it keeps sharing only a vertex that is final already. This checks the field rather than the state of the shared vertex, as the second example shows the vertex can be idle. It fixes all three examples, and the stats of `issue4512.txtar`, which 082c52c57 improved, are unchanged, as are those of every other test in `cue/testdata`. I have a change with a regression test ready to send once the approach is agreed.
