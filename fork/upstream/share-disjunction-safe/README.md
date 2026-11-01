# evaluator: reference meeting a disjunction finalizes a field mid-comprehension, "cyclic reference to field" (v0.18.0-rc.2 regression)

A comprehension in `x` reads `out`, and `out` is `x` unified with a
disjunction that has a reference as a disjunct (`#S | int`). Evaluating `out`
from inside that comprehension finalizes `x` before the comprehension has added
its field, and the field is reported as a cyclic reference.

## Reproducer

```txtar
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

### Expected

`x` and `out` both export as `{"a": 1, "b": 2}`, as with v0.17.1 and v0.18.0-rc.1.

### Actual

```
> exec cue export x.cue
[stderr]
x: cyclic reference to field b:
    ./x.cue:7:10
    ./x.cue:3:1
[exit status 1]
```

## Versions

| Version | Result |
| --- | --- |
| v0.18.0-rc.3 / master | fails |
| v0.18.0-rc.2 | fails |
| v0.18.0-rc.1 | passes |
| v0.17.1 | passes |

Introduced by 082c52c57 ("internal/core/adt: finalize a shared forward
reference before a disjunction"): the reproducer fails at 082c52c57, passes at
its parent 1d5bdcf57, and passes on rc.3 with 082c52c57 reverted.

## Root cause

1. Finalizing the root runs the comprehension of `x` (`processComprehension`).
   Its condition evaluates `out.a`, so `out` is unified while `x` is still
   `evaluating` and the comprehension's task is `taskRUNNING`.
2. `out` shares `x` through the reference `x`. When `#A` resolves to the
   `DisjunctionExpr` `#S | int`, `conjunct.go:scheduleConjunct` calls
   `hasResolver` (true, `#S` is a reference) and then `sharesSingleValue`.
3. Since 082c52c57, `share.go:sharesSingleValue` calls `v.Finalize` on the
   shared vertex (`x`) whenever `shareCycleType == NoCycle`, without checking
   whether `x` is being evaluated.
4. `Finalize` freezes `x`'s scheduler and runs `completeAllArcs`. The arc `b`
   is still `ArcPending`, as the comprehension that adds it is the one on the
   stack, so `composite.go:updateArcType` reports `cyclic reference to field b`
   via `reportFieldCycleError`, and `x` is left finalized with that error.

Checked with a debug build of rc.3: the stack shows
`Vertex.Finalize -> completeAllArcs -> updateArcType` called from
`sharesSingleValue`, under `processComprehension` of `x`.

## Proposed fix

Fork commit `51b9e928f` (`graphops/cue` branch `fix/rc3-share-disjunction-safe`):
`sharesSingleValue` finalizes the shared vertex only if no evaluation it is part
of is in progress. `finalizeInterrupts` treats the vertex as busy if it, its
arcs, or the vertices they share (recursively) have `evalDepth > 0`,
`isCompleting > 0`, or a running task (including pushed-down `parentTasks`), or
if an unfinalized ancestor has a running task. In that case the node is
unshared, as before 082c52c57. Otherwise it keeps the #4512 behaviour, and
the stats of `issue4512.txtar` do not change.
Risk and limits: the check walks the shared vertex's arcs each time a shared
node meets such a disjunction, which costs time on large unfinalized values.
It also depends on the in-progress markers being accurate. A cheaper check, or
deferring the finalization to `finalizeSharing`, may be preferable upstream.

## Related

- #4512 (closed): the exponential sharing case 082c52c57 addresses.
- 4232b8d36 fixed a different failure caused by the same finalization (a field
  `x: #A` dropped as a self-reference, "incomplete value {} | string"). It does
  not fix this case.
- No existing issue found for this case (searched "cyclic reference to field",
  "sharesSingleValue", "finalize shared disjunction", "comprehension
  disjunction regression").
