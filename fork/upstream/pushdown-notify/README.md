# evaluator: field added by a pushed-down comprehension is lost on a struct that references it (v0.17.0 regression)

## Reproducer

```txtar
# a.b and x are unified with each other, so they must be equal.
# The pushed-down comprehension adding x.y must also reach a.b.
exec cue export in.cue
cmp stdout want
-- in.cue --
a: {
	b: x
	if b.on {
		b: {}
	}
}
x: a.b & {on: true}
if x.on {
	x: y: 1
}
-- want --
{
    "a": {
        "b": {
            "on": true,
            "y": 1
        }
    },
    "x": {
        "on": true,
        "y": 1
    }
}
```

### Expected

`a.b` and `x` refer to each other, so both have `y: 1` (output as in `want`).

### Actual

`a.b` has no `y`. Nothing reports an error:

```
> cmp stdout want
--- stdout
+++ want
@@ -1,7 +1,8 @@
 {
     "a": {
         "b": {
-            "on": true
+            "on": true,
+            "y": 1
         }
     },
     "x": {
FAIL: repro.txtar:4: stdout and want differ
```

## Versions

| Version | Result |
| --- | --- |
| v0.18.0-rc.3 / master (a0768bd98) | fails |
| v0.18.0-rc.2 | fails |
| v0.18.0-rc.1 | fails |
| v0.17.1 | fails |
| v0.16.1 | passes |

Bisected to 339485ddf008, "internal/core/adt: dependency-tracking comprehension pushdown" (first released in v0.17.0-alpha.2). Its parent, 376710808, passes.

## Root cause

- `pushDownDeps` (`comprehension.go:231`) turns the root comprehension `if x.on {x: y: 1}` into a parent task of arc `x` (line 267). The root no longer waits for it to reach `fieldConjunctsKnown`.
- Evaluating its guard `x.on` evaluates `x`, then `a.b`, whose conjunct `x` is resolved while that comprehension is still running and has not added `y: 1` yet. The inner guard `b.on` is what makes `a.b` get evaluated at this point. Without it, the export is correct.
- `scheduleVertexConjuncts` (`conjunct.go:466`) copies the conjuncts that `x` has at that moment into `a.b`. It registers `a.b` for later conjuncts only if `x`'s parent has not reached `allAncestorsProcessed|fieldConjunctsKnown` (line 613). The root has reached both, so `a.b` is not registered.
- When the comprehension then adds `y: 1` to `x`, `Vertex.insertConjunct` (`fields.go:262`) forwards it to `x`'s `notify` receivers (line 301). `a.b` is not one of them, so it never gets `y`.
- Even if registration were attempted, `addNotify2` (`conjunct.go:635`) would reject it because `x` itself meets `allAncestorsProcessed`.
- I checked this with a debug print in a scratch build of rc.3. `a.b` copies `x` with 1 conjunct while `x.hasActiveParentTask()` is true, and it does not register.

## Proposed fix

graphops/cue branch `fix/rc3-pushdown-notify`, commit 1ea2a3727:

- In `scheduleVertexConjuncts`, also call `addNotify2` when the arc has an active parent task (`state.hasActiveParentTask()`).
- In `addNotify2`, do not return early on `allAncestorsProcessed` while the node has an active parent task.

Both parts are needed. With only the first one, the reproducer still fails.

This brings back the notification that the parent's `fieldConjunctsKnown` used to imply before pushdown, at the place where that assumption is made. It only affects arcs that a still-running pushed-down comprehension targets. The only cost is extra notifications in those cases.

Limits: the fix covers references resolved through `scheduleVertexConjuncts`. Other places that copy an arc's conjuncts and assume the parent's state means they are complete were not audited.

## Related

None found for this bug. #4422 (hang) and #4520 (default lost in a guard), both closed, were bisected to the same commit, 339485ddf008 (https://cuelang.org/cl/1236923), but have different symptoms.
