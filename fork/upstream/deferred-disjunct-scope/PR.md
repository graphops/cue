# PR: deferred-disjunct-scope

Branch: `fix/deferred-completion-disjunct-scope` (commit cfe089331), based on `cjorge/master` (v0.18.0-rc.3).
Replace `XXXX` with the issue number in the commit message before opening.

## Title

```
internal/core/adt: keep deferred completions within the disjunct reading them
```

## Body (plain text, as in the commit message)

```
Since b68cd7163, the fix for #4510, a field of an inline struct or let
that a task reads before it is complete is completed by a task recorded
on the node of the innermost running task that is not internal, or on
the current task's node when every running task is. That search spans
the whole task stack, including the tasks that were running when the
evaluation of a disjunct started. Those belong to the node the
disjunction is defined on, or to the nodes whose evaluation reached it,
so a field read within a disjunct was completed outside of it:

When no node of the disjunct is rooted, as for a disjunction inside an
inline struct, the completion was recorded on the field whose evaluation
reached the inline struct. An error in a disjunct that is discarded,
such as "field not allowed", then failed that field.

A let read by a struct literal disjunct runs with no task of the
disjunct below it, so the completion was recorded on the node the
disjunction is defined on. It was then cloned into the disjuncts
evaluated after the reading one, which failed instead of it, or checked
a validator of the let against a field they do not have. The reading
disjunct was accepted with a value violating the let's validators.

Only consider the tasks started by the evaluation of the innermost
disjunct, by recording in each overlay frame the depth of the task stack
when the frame is pushed. The fallback to the current task's node then
applies within the disjunct as it does at the top level: it is a let's
copy that a field of the disjunct shares, or a node of the disjunct,
both completed as part of it. Recording the completion on the disjunct
itself instead loses the error, as the disjunct may have run its own
tasks by the time one of its fields reads the let. Recording it on the
outermost task of the disjunct lets an error bypass a guard reading the
let, as described in 47d81c3cb.

Outside of a disjunct nothing changes, and the stats of the tests for
#4510 in nonrooted.txtar are unchanged.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
