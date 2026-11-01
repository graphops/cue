# evaluator: deferred completion of an inline struct or let field escapes the disjunct reading it (v0.18.0-rc.2 regression)

## Reproducer

```txtar
# A disjunction evaluated inside an inline struct: the #B arm of sub fails
# (kind "a" vs "b"), but its error must not fail out. Expected: out is "b".
exec cue export a.cue
cmp stdout want-a

# A let read in one disjunct: L.v violates >1, so the first disjunct must be
# rejected and x is {b: 1}, not the invalid {a: 2}.
exec cue export b.cue
cmp stdout want-b

-- a.cue --
#A: {kind: string, id: kind + "."}
#B: {kind: "b", id: kind + ".", sub: #A | #B}
k: "a"
out: ({x: #A | #B, x: #B & {sub: {id: "a.", kind: k}}}).x.kind
-- want-a --
{
    "k": "a",
    "out": "b"
}
-- b.cue --
let L = {v: 1 & >1, out: v + 1}
x: {a: L.out} | {b: 1}
-- want-b --
{
    "x": {
        "b": 1
    }
}
```

### Expected

`a.cue` exports `out: "b"` (`sub` resolves to `#A`, the `#B` arm is discarded).
`b.cue` exports `x: {b: 1}` (the first disjunct reads `L.out`, whose `L.v` fails `>1`).

### Actual

`a.cue`, a bogus error from the discarded `#B` arm of `sub`:

```
x.sub.kind: field not allowed:
    ./a.cue:4:6
    ./a.cue:2:6
    ./a.cue:4:45
```

`b.cue`, the invalid disjunct is exported silently and the valid one is dropped:

```
{
    "x": {
        "a": 2
    }
}
```

## Versions

| Version | `a.cue` | `b.cue` |
| --- | --- | --- |
| v0.18.0-rc.3 / master (a0768bd98) | fails: "field not allowed" | fails: exports `{a: 2}` |
| v0.18.0-rc.2 | fails: same | fails: exports `{a: 2}` |
| v0.18.0-rc.1 | passes | fails: `incomplete value {a:2} \| {b:1}` |
| v0.17.1 | passes | fails: `incomplete value {a:2} \| {b:1}` |

Both rc.3 outputs first appear at b68cd7163 ("complete nodes read through a let once the referrer
is known", the fix for #4510). Its parent passes `a.cue` and gives the rc.1 output for `b.cue`.
At b68cd7163 itself `a.cue` fails with `x.sub.kind: conflicting values "a" and "b"`, also an
error of the discarded arm. `b.cue` never gave the right answer, but before b68cd7163 the let's
error was ignored in both disjuncts, so export failed as ambiguous. Since b68cd7163 the error
eliminates the valid disjunct and the invalid one is exported without an error.

## Root cause

When a task reads a field of an internal node (inline struct or let copy) that is not finished,
`OpContext.evalStateCI` (context.go) schedules a `handleDeferred` task on
`c.rootedTaskNode()`. `processDeferred` (tasks.go) later completes that field and calls
`ctx.AddBottom` with its error on the node that owns the task. `taskContext.rootedTaskNode`
(sched.go) walks the whole task stack for the innermost non-internal node, ignoring
`OpContext.overlays`. Inside a disjunct whose nodes are all internal, that node lies outside the
disjunct. A debug build of rc.3 shows:

- `a.cue`: the read is `x.sub.kind`, done by the task of `x.sub.id` inside two overlay frames
  (`x` and `x.sub`). The task is recorded on `out`. When it runs, `x.sub.kind` is completed
  in the `#B` arm and "field not allowed" is added to `out`.
- `b.cue`: the read is `L.v`, done by the task of `L.out`. The task is recorded on the original
  `x`, the overlay root, not on the `{a: L.out}` disjunct. The disjunct that later runs it has
  arcs `[b]`, so the `{b: 1}` disjunct gets `L.v`'s error and `{a: 2}` is kept without it.

## Proposed fix

Fork commit 6ba9fee66 (graphops/cue branch `fix/rc3-deferred-disjunct-scope`) stops the search at
the innermost disjunct being evaluated. `pushOverlay` (overlay.go) also records the disjunct's
`nodeContext` and `len(c.stack)` on the overlay frame. `rootedTaskNode`, now on `OpContext`,
looks only at tasks above that depth. If none of them is rooted, it uses the current task's node,
as before. If no task of the disjunct is running, it uses the disjunct, which is completed before
it is accepted. Outside a disjunction nothing changes. The stats of
`cue/testdata/eval/nonrooted.txtar` (the #4510 tests) are unchanged.

Limits: when no rooted task is found inside the disjunct, the work goes on an internal node and
relies on the disjunct's evaluation completing that node. This works for the cases above and for
the fork's test `cue/testdata/eval/deferreddisjunct.txtar`, but I have not shown it holds in
general.

## Related

- #4510 (closed): fixed by b68cd7163, which introduced this.
- No existing issue found for this regression. Searched for disjunction + let, "field not
  allowed" + disjunction, deferred, nonrooted and inline struct + disjunction, and checked the
  issues opened since 2026-10-15.
