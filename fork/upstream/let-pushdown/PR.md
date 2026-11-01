# PR

Branch: `fix/let-comprehension-pushdown` (cjorge-graphops/cue), one commit.

Before opening: replace `XXXX` with the issue number in the commit message and in the header of `cue/testdata/comprehensions/pushdown_let.txtar`, then run `CUE_UPDATE=1 go test ./internal/core/compile/` to refresh the sync mirror.

## Title

internal/core/adt: push down comprehensions with a let in their body

## Body

pushDownDeps counted a let in the body of a comprehension as adding
field conjuncts to the struct the comprehension is in. The comprehension
then no longer counted as pushed down, so a reference resolving a field
of that struct ran it early, evaluating the guards of its clauses. Such
a guard could read a field before another comprehension, still
evaluating its own guard further up the stack, added a conjunct to it.
The field was finalized at its default, and the conjunct arriving later
was dropped without an error. The same comprehension without the let
did not run early.

A let within a comprehension can only be referred to from within the
comprehension, which is why its dependencies are not tracked. For the
same reason, its arc is never reached through iteration or lookup of
the enclosing struct, as 3d1cf46d6 noted when it allowed let arcs in a
frozen field set. Stop counting it as adding field conjuncts.

The let has been counted since 339485ddf, which introduced
dependency-tracking comprehension pushdown. It also defeats the fix for
#4448: with a let added to the body of the guarded comprehension in
issue4448.txtar, that test fails again.

Since e147bfc48, a lookup of a field missing from a struct reports that
the field cannot exist only once no remaining task may declare it. The
let made its comprehension count as such a task. In pushdown.txtar,
letCycleSelfKeyword2 now reports that hosts is an undefined field, as
the same test without the let does, instead of an incomplete "cannot
range over" error. e147bfc48 made the same change to the analogous
golden in nested2.txtar.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
