# evaluator: let in a comprehension body makes a reference run the comprehension early, silently dropping a conjunct

## Reproducer

```txtar
exec cue export x.cue
cmp stdout want

-- x.cue --
x: {
	on: *false | bool
	if on {
		let v = 1
		out: v
	}
	n: 1
}
if x.n == 1 {
	x: on: true
}
-- want --
{
    "x": {
        "out": 1,
        "on": true,
        "n": 1
    }
}
```

### Expected

The test passes: `x.on` is `*false | bool & true`, which is `true`, so the `if on` body adds `out: 1`.
Removing the `let` (`out: 1`) gives this result on master.

### Actual

`cue export` exits 0 and drops the `on: true` conjunct without reporting an error:

```
> exec cue export x.cue
[stdout]
{
    "x": {
        "on": false,
        "n": 1
    }
}
> cmp stdout want
FAIL: repro.txtar:2: stdout and want differ
```

## Versions

| Version | Result |
| --- | --- |
| master / v0.18.0-rc.3 (a0768bd98) | fails |
| v0.18.0-rc.2 | fails |
| v0.18.0-rc.1 | fails |
| v0.17.1 | fails |
| v0.16.1 | passes |

Bisected: 376710808 passes, 339485ddf ("dependency-tracking comprehension pushdown", first in
v0.17.0-alpha.2) fails. There the variant without the `let` fails too. e147bfc48 ("stop lookups
from forcing pushed-down comprehensions", in rc.3) fixed that variant but not this one
(checked at e147bfc48^ and e147bfc48).

## Root cause

`comprehension.go:pushDownDeps` adds `fieldConjunctsKnown` to a comprehension's `completes` for each
`*LetField` in its body. So the `if on` comprehension has `completes` set to
`fieldConjunctsKnown|allTasksCompleted`, not bare `allTasksCompleted`, and `task.isPushedDownComp`
is false. To resolve `x.n` in the top-level guard, `SelectorExpr.resolve` calls
`context.go:OpContext.processRef`, which runs `process(arcTypeKnown|fieldSetKnown, yield)` on `x`.
`sched.go:scheduler.process` selects the tasks that complete `fieldConjunctsKnown`, so it runs the
`if on` comprehension now. The `on: true` conjunct is not there yet, because the top-level
comprehension that adds it is still evaluating its guard. Evaluating `on` finalizes `x.on` at its
default `false`. When the top-level comprehension then inserts `on: true`, `fields.go:Vertex.insertConjunct`
finds `x.on` finalized (`getBareState` returns nil) and returns without adding the conjunct or
reporting an error. I confirmed this sequence with debug prints in a scratch build.
Without the `let`, the comprehension is pushed down (`completes == allTasksCompleted`) and is not
selected, so it runs after `on: true` has arrived. `Vertex.lookup` (unify.go) drives its target with
`valueKnown|fieldConjunctsKnown` in `attemptOnly` mode. From reading the code, it would select such
comprehensions too, but in both reproducers I traced, the forcing went through `processRef`.

## Proposed fix

graphops/cue branch `fix/rc3-let-pushdown`, commit 0d2ce237e: `pushDownDeps` no longer adds
`fieldConjunctsKnown` for a `*LetField` (it still sets `MultiLet`). A let in a comprehension body
can only be referenced from inside that body, so it adds no field conjuncts that anything outside
the comprehension could observe. The existing comment in that case already gives this reason for
not tracking the let's dependencies. The change is one line. With it, such comprehensions count as
pushed down, and `scheduler.mayDeclareArcs` no longer reports that they may declare new arcs.
Because of the second effect, one existing golden changes: in `comprehensions/pushdown.txtar`,
`letCycleSelfKeyword2` (`for hv in s.hosts {let n = s.networks, a: n}` in an open `#E`) now reports
`undefined field: hosts` instead of `cannot range over s.hosts (incomplete)`. The error is still
incomplete and `env` is unchanged. e147bfc48 made the same change to the `issue3990` golden in
`nested2.txtar`. All other tests in `./cue/...` and `./internal/core/...` pass. The fix does not
address the general problem that a node can be finalized while a pushed-down parent task may still
add conjuncts to it. It only stops a let from opting a comprehension out of pushdown.

## Related

- #4448: `cue/testdata/comprehensions/issue4448.txtar` fails again when a `let` is added to the
  body of its guarded comprehension. The `"\(port)"` interpolation then fails with
  "non-concrete value _". In a debug build, `processRef` runs the comprehension early on that
  path too.
- #4520: a different symptom that was also bisected to 339485ddf.
- #4478: a let in a comprehension, but a different bug (fixed by 3d1cf46d6). Its reproducer still
  passes with this fix.
- No existing issue found for this bug.
