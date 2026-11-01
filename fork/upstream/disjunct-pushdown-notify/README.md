evaluator: a field unified with a disjunction silently misses the fields a comprehension adds to it (v0.18.0-alpha.3 regression)

### What version of CUE are you using (`cue version`)?

v0.18.0-rc.3 (also master at a0768bd98).

### Does this issue reproduce with the latest stable release?

No. It passes on v0.17.1. It first fails in v0.18.0-alpha.3, and still
fails on v0.18.0-rc.1, rc.2 and rc.3. It also fails with
`CUE_DEBUG=sharing=0`.

### What did you do?

```
exec cue export in.cue
cmp stdout want

-- in.cue --
x: {
	a: 1
	for _, v in out {b: v}
}
out: x & ({...} | int)
-- want --
{
    "x": {
        "a": 1,
        "b": 1
    },
    "out": {
        "a": 1,
        "b": 1
    }
}
```

### What did you expect to see?

A passing test. `out` unifies `x`, and only the struct disjunct can apply,
so `out` has every field of `x`, including `b`.

### What did you see instead?

```
> cmp stdout want
--- stdout
+++ want
@@ -4,6 +4,7 @@
         "b": 1
     },
     "out": {
-        "a": 1
+        "a": 1,
+        "b": 1
     }
 }
FAIL: repro.txtar:2: stdout and want differ
```

`out` has no `b`, and no error is reported. The comprehension does not need
to be in another field: `out: ({...} | int) & {a: 1, for _, v in out {b: v}}`
also exports `out: {a: 1}`. And a guard testing such a field for absence,
`out: ({...} | int) & {a: 1, if out.b == _|_ {b: 1}}`, exports `{a: 1}`
on rc.3. Without the disjunction, it reports "cyclic reference to field b".
With it, v0.17.1 to rc.2 also report a cycle ("circular dependency in
evaluation of conditionals").

### Analysis

Bisected to d47e4cee8b1b, "internal/core/adt: settle a pending arc once its
comprehension is done" (Updates #4487). Its parent, 6213533685d1, passes.
The guard variant first fails at e147bfc48a33, "internal/core/adt: stop
lookups from forcing pushed-down comprehensions".

Since https://cuelang.org/cl/1236923, the comprehension is a parent task of
the pending arc `b` it creates (`pushDownDeps`). When `out` meets the
disjunction, the comprehension is deferred until the disjunction has cloned
`out` for each disjunct (`hasPendingDisjunction` in `scheduler.process`),
so that the clone of the comprehension runs in each disjunct. However,
`overlayContext.cloneScheduler` (internal/core/adt/overlay.go) does not copy
`parentTasks`, so the clone of `b` has no parent task. In the disjunct,
the cloned comprehension iterates over `out`, which evaluates the arc types
of `out` first. Since d47e4cee8, `stateCompletions` settles a pending arc
with no active parent task as `ArcNotPresent`, so `b` is dropped before
the comprehension adds to it. Since e147bfc48, a lookup likewise runs only
the parent tasks of the arc it reads, so the guard finds `b` absent. A
debug build confirms that the arc is settled in the disjunct with
`len(parentTasks) == 0`.

This is another case of the family #4487 and #4520 belong to, where a field
is taken as complete while a pushed-down comprehension targeting it is
pending. Here the reason is that the link between the two is lost when
cloning for a disjunct.

### Proposed approach

Carry `parentTasks` over when cloning a scheduler for a disjunct, and point
them at the clones of the tasks. An arc is cloned before the tasks of its
parent, so `cloneTask` would record each cloned comprehension task in a map
on the `overlayContext`, and `cloneRoot` would rewrite the parent tasks of
every cloned node after cloning, as it already does for each task's node.
A parent task that was not cloned, because it belongs to a node outside the
disjunct, adds to the original arc and not to the clone. It is left out, as
it is now.

The map is only allocated when a comprehension task is cloned. None of the
existing tests change, including their stats. I have a change with a
regression test ready if this approach is OK.

A different case that this does not fix is a reference to a field that a
comprehension adds to later, from a field that meets a disjunction, as in
`out: x.args & ({...} | int)` with `x: {args: a: 1, if out.a == 1 {args: b:
2}}`. There the disjunction of `out` is resolved before the comprehension
adds `b` to `x.args`, so the fields that reach `out` afterwards are lost.
That case has failed since v0.17.0-alpha.2 (339485ddf), and I will report
it separately.
