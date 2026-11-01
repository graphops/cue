# evaluator: error below a function argument loses its path, so "field not allowed" names no field and distinct errors merge

## Reproducer

```txtar
# An error below a function argument is reported at the field holding the
# call, but should still name its path within the argument: each rejected
# field is named, and distinct errors are not merged into one.
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

### Expected

Two errors, one per rejected field, each saying which field it rejects. The
exact format above (call-site path, then the path within the argument starting
at the parameter) is what the proposed fix prints; any form that names `b` and
`c` separately would do.

### Actual

```
out: field not allowed:
    ./x.cue:5:6
    ./x.cue:5:15
```

The message does not say which field is not allowed, and the error for `c` is
gone.

## Versions

- v0.18.0-rc.3 (== master a0768bd98): fails, as above.
- v0.18.0-rc.2: fails, same output.
- v0.18.0-rc.1: both errors reported and named, as `x.b: field not allowed` and
  `x.c: field not allowed` (the parameter's own path, before errors were moved
  to the call site). Fails the exact `cmp` only because the `out:` prefix is new.
- v0.17.1: n/a (no `functions` experiment).
- Introduced by 7bf42f62f ("internal/core/adt: report errors arising in a call
  at the calling field"): its parent 71b642169 prints the two `x.b` / `x.c`
  errors, 7bf42f62f prints the single `out: field not allowed` (both built from
  source; dev builds need `CUE_VERSION_OVERRIDE=v0.18.0` for the experiment).

## Root cause

`expr.go:(*FuncValue).call` (and `expr.go:(*Builtin).applyParamTypes` for
builtin signatures) passes a failed argument to `funcsig.go:(*OpContext).atCallSite`,
which rewrites each error with `funcsig.go:moveToCallSite`. For an error in the
arguments, `funcsig.go:callSiteOf` returns `below == nil` (`return nil, true, true`),
discarding the labels it collected between the failing vertex and the argument,
and `moveToCallSite` then sets `altPath = nil` and `site = site`. So
`errors.go:(*baseError).Path` yields only the calling field (`out`). A closedness
error's message is just "field not allowed": the rejected field was only ever
named by the path (`x.b`), which is now gone. Both errors also share the same
position (5:6), path and message, so `cue/errors/errors.go:(*list).removeMultiples`
(via `Sanitize`) keeps one of them. Checked with debug prints: two errors leave
`atCallSite`; `removeMultiples` gets a group of 2 and keeps 1.

## Proposed fix

graphops/cue branch `fix/rc3-call-closedness-path` (65363248a, 7fe4e1cf4).
`callSiteOf` returns the labels below the argument too; `moveToCallSite` builds
the path within the argument (parameter name + labels, plus the path from a call
nested in the argument) and stores it as `baseError.argPath`, which
`ValueError.Msg`/`ConflictError.Msg` put in front of the message. It is set only
for errors below the argument, so an error at the argument itself (`f(1)`) reads
as now. `atCallSite` takes the parameter name for the builtin case, where `args`
is the single argument vertex. The second commit handles wrapping errors (failed
validators, interpolations) with a `callSiteWrapper` that prints the path once, in
front of the outermost message. Limits: the reported path stays the call site,
so the argument path lives in the message text (a format choice upstream may want
to make differently); only `ValueError`, `ConflictError` and such wrappers carry it.

## Related

- #4335 (open): make "field not allowed" errors friendlier. Same symptom class,
  not this regression.
- #4270 (closed): error deduplication removed distinct errors at the same
  position; dedup now compares messages, which this regression defeats by
  making the messages identical.
- No existing issue or CL found for this regression.
