evaluator: a field unifying another one silently misses a field a comprehension adds to it when the comprehension's guard reads the first field (v0.17.0 regression)

### What version of CUE are you using (`cue version`)?

v0.18.0-rc.3 (also master at a0768bd98).

### Does this issue reproduce with the latest stable release?

Yes. It fails on v0.17.1 and passes on v0.16.1. It first fails in
v0.17.0-alpha.2.

### What did you do?

```
exec cue export in.cue
cmp stdout want

-- in.cue --
a: x & {z: 1}
x: on: true
if a.on {
	x: y: 1
}
-- want --
{
    "a": {
        "on": true,
        "y": 1,
        "z": 1
    },
    "x": {
        "on": true,
        "y": 1
    }
}
```

### What did you expect to see?

A passing test. `a` unifies `x`, so it has every field of `x`, including
`y`.

### What did you see instead?

```
> cmp stdout want
--- stdout
+++ want
@@ -1,6 +1,7 @@
 {
     "a": {
         "on": true,
+        "y": 1,
         "z": 1
     },
     "x": {
FAIL: repro.txtar:2: stdout and want differ
```

`a` has no `y`, and no error is reported.

### Analysis

Bisected to 339485ddf008, "internal/core/adt: dependency-tracking
comprehension pushdown" (https://cuelang.org/cl/1236923). Its parent,
376710808d8a, passes.

Since that commit, the comprehension does not hold up the root struct.
It is a parent task of `x` instead. Its guard evaluates `a`, which
resolves `x` while the comprehension is still running.
`nodeContext.scheduleVertexConjuncts` (internal/core/adt/conjunct.go)
copies the one conjunct `x` has at that point. It registers `a` for later
conjuncts only while the parent of `x` has not reached
`fieldConjunctsKnown`. The root struct has reached it, so `a` never
receives `y: 1`. If `x` is itself being evaluated at that
point, as when the guard reads `x`, `nodeContext.addNotify2` rejects the
registration too, because `x` has all its ancestors processed.

#4520 and #4487 were bisected to the same commit and have the same shape,
with different symptoms. In all three, a pushed-down comprehension that
targets a field is still pending or running, and some code treats the
field as complete because its parent is. Both are fixed on master
(b218faaa6 and d47e4cee8), but those changes do not cover this case.

#4524 has a similar shape: `m1: c1 & {y: 3}`, with
`if c1.x == 2 {c1: #B}` adding to `c1` under explicitopen. It is a
different bug. It still fails with the change below, and without
explicitopen (`c1: {x: 2, ...}`) the same shape passes on master. The same
miss does affect closedness when the guard reads the referring field.
With `m1: c1 & {y: 3}`, `c1: {x: 2, ...}` and `if m1.x == 2 {c1: #B}`,
master accepts `m1.y`.

### Proposed approach

Treat an active parent task as a reason to register, in both places:

- In `scheduleVertexConjuncts`, also register when the arc has an active
  parent task (`hasActiveParentTask`).
- In `addNotify2`, do not skip the registration because of
  `allAncestorsProcessed` while the node has an active parent task.

This brings back the notification that the parent's `fieldConjunctsKnown`
implied before pushdown. It only affects arcs targeted by a pushed-down
comprehension that has not finished yet. In the existing tests, the
only change is a few more notifications in `functions/tighten.txtar`. I
have a change with a regression test ready if this approach is OK.
