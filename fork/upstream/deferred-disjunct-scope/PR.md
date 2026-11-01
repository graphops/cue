# PR: deferred-disjunct-scope

Branch: cjorge-graphops/cue `fix/deferred-completion-disjunct-scope`: commit 4b0c3e44f plus fixup 01a3f7077, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`). Based on `cjorge/master` a0768bd98 (upstream master, v0.18.0-rc.3).

Before opening, replace `XXXX` with the issue number in the commit message and in: `cue/testdata/eval/nonrooted_disjunct.txtar`, `internal/core/compile/testdata/sync/eval/nonrooted_disjunct.txtar`.

## Title

```
internal/core/adt: keep deferred completions within the disjunct reading them
```

## Body (plain text, as in the commit message)

```
Since the fix for #4510, a field of an inline struct or let that a task
reads before it is complete is completed by a task recorded on the node
of the innermost running task that is not internal, or on the current
task's node when every running task is. That search spans the whole task
stack, including the tasks that were running when the evaluation of a
disjunct started. Those belong to the node the disjunction is defined
on, or to the nodes whose evaluation reached it, so a field read within
a disjunct was completed outside of it:

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
let, as described in the fix for #4532.

Outside of a disjunct nothing changes, and the stats of the tests for
#4510 in nonrooted.txtar are unchanged.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

## Reviewer note

`rootedTaskNode` returns nil when no task was started within the innermost
disjunct. This is the existing "no task is running" case, as before the
change, now scoped to the disjunct. No test reaches it: in every case tried,
a task of the disjunct is running when one of its fields reads an unfinished
inline value. Returning the node of an earlier task instead, as the previous
fallback did, would record the work outside the disjunct, which is the bug
fixed here.
