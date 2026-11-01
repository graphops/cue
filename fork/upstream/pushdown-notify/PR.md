Title:

internal/core/adt: notify a reference of conjuncts a pushed-down comprehension adds

Body (plain text, the commit message; replace #XXXX with the filed issue number):

```
A reference to a field copies the conjuncts the field has so far, and
registers for those added later only while the field's parent has not
reached fieldConjunctsKnown. Since the dependency-tracking comprehension
pushdown in commit 339485ddf008, a comprehension declaring the field no
longer holds up the parent: it is a parent task of the field instead. A
reference resolved while that task was still pending or running,
typically because the comprehension's own guard read the referring
field, did not register, so the conjuncts the comprehension added
afterwards never reached it. The referring field then silently lacked
fields the referenced one has.

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

Branch: fix/pushdown-notify-reference in ~/code/graphops/cue (commit 1ded15b6b), based on cjorge/master (a0768bd98).
