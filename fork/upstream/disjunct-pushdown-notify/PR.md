Title:

internal/core/adt: keep the parent tasks of an arc cloned for a disjunct

Body (plain text, the commit message; replace #XXXX with the filed issue number):

```
A comprehension in a struct is a parent task of the pending arcs it
creates for the fields its body declares; see pushDownDeps. When the
struct is unified with a disjunction, the comprehension is deferred
until the disjunction has cloned the struct for each disjunct, so that
it runs in each of them. cloneScheduler did not copy the parent tasks of
the arcs to their clones though, so in a disjunct nothing was left that
could still declare those fields.

Since commit d47e4cee8b1b, "internal/core/adt: settle a pending arc once
its comprehension is done", a pending arc without an active parent task
is settled as absent. In a disjunct, that now happens as soon as the arc
is evaluated before the comprehension has run, for instance because the
comprehension iterates over the struct it is in. The fields the
comprehension adds afterwards are dropped, so the result silently lacks
fields which the same struct has without the disjunction. Since
https://cuelang.org/cl/1257111, a lookup likewise runs only the parent
tasks of the arc it reads, so a guard testing whether such a field is
absent finds it absent in a disjunct, instead of reporting the cycle it
reports without the disjunction.

Copy the parent tasks when cloning a scheduler, and point them at the
clones of the tasks once the whole disjunct is cloned, as an arc is
cloned before the tasks of its parent. A parent task that was not
cloned, as it belongs to a node outside the disjunct, adds conjuncts to
the original arc rather than to its clone, so it is left out as before.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

Commit: 84ed95ccdd14 on branch fix/disjunct-pushdown-notify, based on cjorge/master = upstream master a0768bd98. Files: internal/core/adt/overlay.go (+24), cue/testdata/comprehensions/pushdowndisjunct.txtar (new), internal/core/compile/testdata/sync/comprehensions/pushdowndisjunct.txtar (generated).
