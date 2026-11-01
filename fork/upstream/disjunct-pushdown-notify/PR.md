# PR: disjunct-pushdown-notify

Branch: cjorge-graphops/cue `fix/disjunct-pushdown-notify`: commit d751a12c5 plus fixup 6e45da789, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`). Based on `cjorge/master` a0768bd98 (upstream master, v0.18.0-rc.3).

Before opening, replace `XXXX` with the issue number in the commit message and in: `cue/testdata/comprehensions/pushdowndisjunct.txtar`, `internal/core/compile/testdata/sync/comprehensions/pushdowndisjunct.txtar`.

## Title

```
internal/core/adt: keep the parent tasks of an arc cloned for a disjunct
```

## Body (plain text, as in the commit message)

```
A comprehension in a struct is a parent task of the pending arcs it
creates for the fields its body declares; see pushDownDeps. When the
struct is unified with a disjunction, the comprehension is deferred
until the disjunction has cloned the struct for each disjunct, so that
it runs in each of them. cloneScheduler did not copy the parent tasks of
the arcs to their clones though, so in a disjunct nothing was left that
could still declare those fields.

Since the change for #4487 which settles a pending arc once its
comprehension is done, a pending arc without an active parent task is
settled as absent. In a disjunct, that now happens as soon as the arc is
evaluated before the comprehension has run, for instance because the
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

## At filing time

The branch is now d751a12c5 plus fixup 6e45da789 (squash before sending; the
"Commit: 84ed95ccdd14" line above is stale). The test's `args` case documents
the separate, unfixed `out: x.args & ({...} | int)` bug with
`@test(eq, {a: 1}, incorrect) @test(eq:todo, {a: 1, b: 2}, p=1)` and says it
"is reported separately": once that issue is filed, add its
`https://cuelang.org/issue/NNNN` link to the case's comment.
