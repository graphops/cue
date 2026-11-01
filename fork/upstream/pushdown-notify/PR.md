# PR: pushdown-notify

Branch: cjorge-graphops/cue `fix/pushdown-notify-reference`: commit 1551f4777 plus fixup 0f75b23da, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`). Based on `cjorge/master` a0768bd98 (upstream master, v0.18.0-rc.3).

Before opening, replace `XXXX` with the issue number in the commit message and in: `cue/testdata/comprehensions/pushdownnotify.txtar`, `internal/core/compile/testdata/sync/comprehensions/pushdownnotify.txtar`.

## Title

```
internal/core/adt: notify a reference of conjuncts a pushed-down comprehension adds
```

## Body (plain text, as in the commit message)

```
A reference to a field copies the conjuncts the field has so far, and
registers for those added later only while the field's parent has not
reached fieldConjunctsKnown. Since the dependency-tracking comprehension
pushdown in https://cuelang.org/cl/1236923, a comprehension declaring
the field no longer holds up the parent: it is a parent task of the
field instead. A reference resolved while that task was still pending or
running, typically because the comprehension's own guard read the
referring field, did not register, so the conjuncts the comprehension
added afterwards never reached it. The referring field then silently
lacked fields the referenced one has.

Also register while the field has an active parent task. addNotify2
would still reject the registration if the field was being evaluated
already, as it then has all its ancestors processed, so accept it there
in the same case too. Both are needed: the test covers a reference
copying a field that is not being evaluated and one copying a field
that is.

The only other effect on existing tests is a few more notifications in
functions/tighten.txtar, where a function's result type refers to a
definition that a comprehension refines after the call.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

## At filing time

The branch now carries a fixup commit (0f75b23da) on top of 1551f4777 (the
"1ded15b6b" above is stale); squash it in with `git rebase -i --autosquash
cjorge/master`. It adds a `https://cuelang.org/issue/XXXX` line to the header
of `pushdownnotify.txtar`: replace `XXXX` there too, then run
`CUE_UPDATE=1 go test ./internal/core/compile/` to refresh the sync mirror.

The test paragraph of the commit message stays accurate. Optionally extend it:

    Both are needed: the test covers a reference copying a field that is
    not being evaluated and one copying a field that is. The same miss let
    the reference stay open when the comprehension adds a definition.

The tighten.txtar notifications come from the `admission` cases whose call
result refers to a definition that a later comprehension, guarded on the
call's result, narrows (ov, pc, qc, wc, lc); their results do not change.
