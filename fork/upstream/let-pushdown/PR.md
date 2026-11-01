# PR: let-pushdown

Branch: cjorge-graphops/cue `fix/let-comprehension-pushdown`: commit 9c36f394a plus fixup 44e8aa23a, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`). Based on `cjorge/master` a0768bd98 (upstream master, v0.18.0-rc.3).

Before opening, replace `XXXX` with the issue number in the commit message and in: `cue/testdata/comprehensions/pushdown_let.txtar`, `internal/core/compile/testdata/sync/comprehensions/pushdown_let.txtar`.

## Title

```
internal/core/adt: push down comprehensions with a let in their body
```

## Body (plain text, as in the commit message)

```
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
same reason, its arc is never reached through iteration or lookup of the
enclosing struct, as the fix for #4478 noted when it allowed let arcs in
a frozen field set. Stop counting it as adding field conjuncts.

The let has been counted since https://cuelang.org/cl/1236923, which
introduced dependency-tracking comprehension pushdown. It also defeats
the fix for #4448: with a let added to the body of the guarded
comprehension in issue4448.txtar, that test fails again.

Since https://cuelang.org/cl/1257111, a lookup of a field missing from a
struct reports that the field cannot exist only once no remaining task
may declare it. The let made its comprehension count as such a task. In
pushdown.txtar, letCycleSelfKeyword2 now reports that hosts is an
undefined field, as the same test without the let does, instead of an
incomplete "cannot range over" error. That change made the same change
to the analogous golden in nested2.txtar.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

## At filing time

The branch now carries a fixup commit (44e8aa23a) with more cases in
`pushdown_let.txtar`; squash it into the main commit (`git rebase -i
--autosquash cjorge/master`). The test header's `XXXX` and the sync mirror
need the issue number as above.

In the commit message (and the body above), replace the paragraph naming
#4448 with:

    The let has been counted since https://cuelang.org/cl/1236923, which
    introduced dependency-tracking comprehension pushdown. It also defeats
    the fixes for #4448 and #4487: with a let added to the body of the
    guarded comprehension of their tests, they fail again. The test covers
    both, and records that a comprehension embedding a let in its body is
    still not pushed down and still drops the conjunct, as with a dynamic
    field.
