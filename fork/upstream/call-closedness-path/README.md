evaluator: an error below a function argument no longer names its path, so "field not allowed" names no field and distinct errors merge

### What version of CUE are you using (`cue version`)?

v0.18.0-rc.3 (also master at a0768bd98).

### Does this issue reproduce with the latest stable release?

No: v0.17.1 has no `functions` experiment. It is a regression between
v0.18.0-rc.1 (good) and v0.18.0-rc.2 (bad).

### What did you do?

```
! exec cue vet x.cue
cmp stderr want

-- x.cue --
@experiment(functions)

#C: {a: int}
f: func(x: #C) -> int: 1
out: f({a: 1, b: 2, c: 3})
-- want --
out: x.b: field not allowed:
    ./x.cue:5:6
    ./x.cue:5:15
out: x.c: field not allowed:
    ./x.cue:5:6
    ./x.cue:5:21
```

### What did you expect to see?

Two errors, one per rejected field, each naming the field it rejects. The
exact format above (the calling field, then the path within the argument
starting with the parameter) is the one the proposed fix prints; any form
that names `b` and `c` separately would do.

v0.18.0-rc.1 printed `x.b: field not allowed` and `x.c: field not allowed`
(the parameter's path, before errors were moved to the calling field).

### What did you see instead?

```
out: field not allowed:
    ./x.cue:5:6
    ./x.cue:5:15
```

The message does not say which field is not allowed, and the error for `c`
is gone.

### Analysis

Bisected to 7bf42f62f ("internal/core/adt: report errors arising in a call
at the calling field"); its parent 71b642169 prints the two `x.b` / `x.c`
errors. That change deliberately reports an argument error at the calling
field without the parameter's name. But in `funcsig.go`, `callSiteOf` returns
no labels for an error at or below an argument, and `moveToCallSite` clears
`altPath`, so an error below an argument also loses its path within the
argument. A closedness error's message is just "field not allowed": the field
was only ever named by the path. Both errors then share position (5:6), path
and message, so `cue/errors` `removeMultiples` (see #4270) keeps one of them.
The same happens for argument errors from signatures attached to a builtin
(`applyParamTypes`), and for any error below an argument (bounds, conflicts,
validators). This is unrelated to the comprehension pushdown changes.

### Proposed approach

Keep reporting every argument error at the calling field, as 7bf42f62f
intends, and leave an error at the argument itself (`f(1)`) as it is now.
For an error below the argument, have `moveToCallSite` also return the path
within the argument (parameter, then the labels down to the failing field,
plus the path below a nested call's site), and have `atCallSite` wrap the
moved error in an error carrying that path as its message, the way other
wrapping errors such as "invalid default for parameter x" do. The reported
path and position stay those of the calling field, so only the message
changes: `out: x.b: field not allowed`. Putting the argument path into
`Path()` instead (e.g. `out.x.b`) would name a path that does not exist and
would be ambiguous with an error in the call's result. A fix along these
lines is ready (+49/-30 lines in internal/core/adt, plus tests).

How the inner path is presented is the part most open to discussion: #2782
is the same question for builtins. There, https://cuelang.org/cl/1238666
(joining the path onto `Path()`) was reverted, and its follow-up
https://cuelang.org/cl/1238752 was abandoned as superseded by a newer
change. If that newer work settles how inner paths of wrapped errors are
reported, we would rather follow it. The regression itself, the field
not named and distinct errors merged into one, holds whichever form is
chosen.

### Related

- #4270: since its fix, distinct messages at one position are kept. Here
  they merge because the lost path makes the two errors identical.
- #2782, #3186: the same missing inner path for errors from the builtins
  `json.Validate` and `json.Marshal` ("field not allowed" names no
  field). The proposed fix does not change them. See above for the
  reverted and abandoned changes for #2782.
- #4335: asks for "field not allowed" messages that say what closed the
  struct.
