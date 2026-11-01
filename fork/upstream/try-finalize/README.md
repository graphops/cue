# evaluator: try clause whose expression reaches the arcs its body sets drops the body (constraint lost, or spurious cyclic reference)

When the expression of a `try` clause refers to a field that the clause's own
body also contributes to, the body's conjuncts are lost. A constraint is
dropped without an error (`drop.cue`), and a new field is reported as a cyclic
reference (`cyc.cue`).

`try` is experimental: it needs `@experiment(try)` in the file (preview since
v0.16.0; inside a module, `language: version` must be at least `v0.16.0`).

## Reproducer

```txtar
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

### Expected

`drop.cue` fails with `x.a: invalid value 5 (out of bound <=2)`, and `cyc.cue`
exports `{"x": {"a": 5, "b": 1}}`.

### Actual

`drop.cue` exports without error, and the constraint is not applied:

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

## Versions

| Version | Result |
| --- | --- |
| v0.18.0-rc.3 (= master) | fails (both) |
| v0.18.0-rc.2 | fails (both) |
| v0.18.0-rc.1 | fails (both) |
| v0.17.1 | fails (both) |
| v0.16.1 | passes |

Bisected to 339485ddf ("internal/core/adt: dependency-tracking comprehension
pushdown", first released in v0.17.0-alpha.2): its parent passes, 339485ddf fails.

## Root cause

1. `pushDownDeps` (comprehension.go) creates the body's arcs `x`, `x.a`/`x.b`
   as pending and adds the try comprehension to their `parentTasks`.
2. `TryClause.yield` (expr.go) pre-evaluates `x?` in an inline vertex and calls
   `v.Finalize(c)` on it. That inline vertex shares `x`, so `Vertex.unify`
   (unify.go, sharing branch, `w.unify(...)` with mode `finalize`) and
   `finalizeSharing` (share.go) finalize `x`. Its `completeAllArcs` then
   finalizes `x.a`/`x.b`, while the try comprehension is still `taskRUNNING`
   and has not yielded its body.
3. When the body is then yielded, `Vertex.insertConjunct` (fields.go) gets a nil
   state from `getBareState` because `x` is finalized, and returns without
   adding the conjunct. `{a: <=2}` is lost silently.
4. For `cyc.cue`, `x.b` is still `ArcPending` when `completeAllArcs` reaches it.
   `updateArcType(ArcNotPresent)` sees `arcTypeKnown` frozen and calls
   `reportFieldCycleError` (composite.go), which reports the cyclic reference.

I confirmed each step with debug prints in a scratch build of rc.3. Before
339485ddf, pushdown materialized the body's fields early, so nothing was left
for the pre-evaluation to finalize ahead of the yield.

## Proposed fix

Fork commit `e220d925d` (graphops/cue branch `fix/rc3-try-finalize`): on entry
to `Vertex.unify`, a `finalize` request is lowered to `attemptOnly` when one
of the node's `parentTasks` is a running comprehension with a try clause
(`nodeContext.awaitsRunningTry`). The node is finalized later, after the
comprehension completes. One check in `unify` covers every path that reaches
these arcs (sharing, `finalizeSharing`, `completeAllArcs`). The fix is
limited to try clauses on purpose. Doing the same for every running parent
task hides real guard cycles, for example `cycle/compbottom2.txtar`. `go test
./internal/core/... ./cue/...` passes with the fix. Limits: during the
pre-evaluation, the try's expression now sees those arcs before they are
final. The fix also adds a scan of `parentTasks` to each `finalize` call on a
node that has parent tasks.

## Related

- #4485 (closed): `try { x: b: x.a? }`, a try body reading an arc it also
  sets. This is a different shape, and the fix for it (5e2823acf) is in rc.3.
- #4486 (open): spurious cyclic reference from a `try` nested in another
  `try`'s body. This is a different trigger: the inner try's target probe.
- No existing issue was found for this shape.
