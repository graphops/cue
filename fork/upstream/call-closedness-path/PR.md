Branch: cjorge-graphops/cue fix/call-error-argument-path (one commit, b0188082c), based on master a0768bd98.
Replace #XXXX with the issue number before opening the PR; the PR body is the commit message body.

Title:

internal/core/adt: name the path within an argument of a call error

Body:

Since 7bf42f62f, an error arising in a function argument is reported at
the field holding the call, without its path within the argument. That
is the intent for an error at the argument itself, as in "w:
conflicting values 1.5 and int" for w: f(1.5), but an error below the
argument no longer says which part of it failed. A closedness error
does not name the field it rejects at all, and distinct errors at the
same call now share their position, path and message, so all but one
of them are dropped as duplicates when the errors are printed (see
#4270). For example, with

	#C: {a: int}
	f: func(x: #C) -> int: 1
	out: f({a: 1, b: 2, c: 3})

cue vet reported a single "out: field not allowed", where v0.18.0-rc.1
reported "x.b: field not allowed" and "x.c: field not allowed".

atCallSite now wraps an error below an argument, rather than at the
argument itself, in an error naming its path within the argument,
starting with the parameter, as in "out: x.b: field not allowed". As
for the other wrapping errors, the error is still reported at the path
and position of the error it wraps, so only the message changes. The
path covers an error moved out of a call nested in the argument, and
precedes the message of an error that wraps the failing one, such as a
failed validator. Argument errors from the signatures attached to a
builtin are named the same way.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>

