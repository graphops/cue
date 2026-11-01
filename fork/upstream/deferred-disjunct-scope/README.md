Title: evaluator: an error in a let or inline struct read by a disjunct escapes that disjunct (v0.18.0-rc.2 regression)

### What version of CUE are you using (`cue version`)?

<pre>
$ cue version
cue version v0.18.0-rc.3

CUE language version v0.18.0
</pre>

Also reproduced with the current master (a0768bd98).

### Does this issue reproduce with the latest stable release?

No for `a.cue`: v0.17.1 and v0.18.0-rc.1 export `"out": "b"`. It fails since v0.18.0-rc.2.

`b.cue` never gave the right result, but differently: v0.17.1 and v0.18.0-rc.1 fail with `x: incomplete value {a:2} | {b:1}`, as the let's error is not checked at all. Since v0.18.0-rc.2 the export succeeds with the invalid disjunct.

Bisected to https://github.com/cue-lang/cue/commit/b68cd7163 (the fix for #4510), "internal/core/adt: complete nodes read through a let once the referrer is known". Its parent passes `a.cue` and gives the rc.1 error for `b.cue`.

### What did you do?

```
# The #B arm of x.sub conflicts on kind and is discarded, so out is "b".
exec cue export a.cue
cmp stdout want-a

# L.v violates >1, so the first disjunct fails and x is {b: 1}.
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

### What did you expect to see?

A passing test. In `a.cue`, `x.sub` can only be `#A`, as `#B` requires kind `"b"`, and so `x` is `#B` and `out` is `"b"`. In `b.cue`, the first disjunct reads `L.out`, and `L.v` fails its validator, so the first disjunct is eliminated, as `r: L.out` alone fails since #4510.

### What did you see instead?

`a.cue` fails with an error of the discarded `#B` arm of `x.sub`:

```
x.sub.kind: field not allowed:
    ./a.cue:4:6
    ./a.cue:2:6
    ./a.cue:4:45
```

`b.cue` exports the disjunct that violates the let's validator:

```
{
    "x": {
        "a": 2
    }
}
```

### Analysis

Since b68cd7163, when a task reads a field of an inline struct or let copy that is not complete, `OpContext.evalStateCI` schedules a `handleDeferred` task on `rootedTaskNode()`: the node of the innermost running task that is not internal, or the current task's node when all are internal. That search covers the whole task stack, including the tasks that were running when the evaluation of a disjunct started, which belong to the node the disjunction is defined on, or to the fields whose evaluation reached it. A debug build shows:

- `a.cue`: `x` and `x.sub` are in an inline struct, so no node of either disjunction is rooted. `x.sub.id` of the `#B` arm reads `x.sub.kind`, and the completion is recorded on `out`, which then fails with that arm's error.
- `b.cue`: the disjunct's field `a` shares the let copy `L.out`, whose task reads `L.v`. The only rooted task is the one evaluating the disjunction of `x`, so the completion is recorded on `x` itself. It is cloned into the `{b: 1}` disjunct, evaluated next, which fails, while `{a: 2}` was already accepted without the error.

The same happens to a let read within a disjunct inside an inline struct: its error then fails the whole field, or eliminates the wrong disjunct.

This is not related to the comprehension pushdown of 339485ddf: no comprehension is involved.

### Proposed approach

Within a disjunct, consider only the tasks started by the evaluation of that disjunct: record the depth of the task stack in each overlay frame (`pushOverlay`), and have `rootedTaskNode` search above the innermost frame's depth. The existing fallback to the current task's node then applies within the disjunct as it does at the top level: a let copy that a field of the disjunct shares, or a node of the disjunct, both completed as part of it. Outside of a disjunct nothing changes, and the stats of `nonrooted.txtar` are unchanged.

I tried two alternatives, which each fail a case: recording the completion on the disjunct itself loses the error in `b.cue`, as the disjunct may have run its own tasks by the time its field reads the let, and recording it on the outermost task of the disjunct lets an error bypass a guard (`if names[k] != _|_`) of the let reading it, the problem described in 47d81c3cb (#4532). I have a change with a regression test ready to send once the approach is agreed.
