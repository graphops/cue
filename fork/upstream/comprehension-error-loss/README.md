# evaluator: error in a comprehension condition is lost for a field declared next to it

## Reproducer

```txtar
# A field declared next to a comprehension (x: [...]) keeps its declared
# value when the comprehension's condition cannot be evaluated, instead of
# reporting the condition's error.

# The condition never becomes concrete: out is incomplete, not [].
! exec cue export incomplete.cue
! stdout .
stderr 'non-concrete value int in operand to =='

# The condition is an error: out is that error, not [].
! exec cue export -e out fatal.cue
! stdout .
stderr 'conflicting values 2 and 1'

-- incomplete.cue --
out: #P.x
#P: {
	x: [...]
	if _b == 1 {
		x: [1]
	}
}
_b: int
-- fatal.cue --
out: p.x
p: {
	x: [...]
	if b {
		x: [1]
	}
}
b: 1 & 2
```

### Expected

Both commands fail. `incomplete.cue`: the guard of `x` cannot be decided, so `out` is
incomplete (`non-concrete value int in operand to ==`), as in v0.16.1. `fatal.cue`: the
guard is an error, so `out` is that error (`conflicting values 2 and 1`).

### Actual

On v0.18.0-rc.3 (= master) both exit 0 and treat the condition as false:

```
> ! exec cue export incomplete.cue
[stdout]
{
    "out": []
}
FAIL: repro.txtar:6: unexpected command success
```

```
$ cue export -e out fatal.cue
[]
$ echo $?
0
```

Without `x: [...]` both report an error (`undefined field: x`), and a plain `cue export
fatal.cue` reports the error of `b`; only the reference to the declared field sees `[]`.
`cue vet -c incomplete.cue` also exits 0.

## Versions

| Version | `incomplete.cue` | `fatal.cue` (`-e out`) | with `else` (`@experiment(try)`) |
| --- | --- | --- | --- |
| v0.18.0-rc.3 / master | fails | fails | fails |
| v0.18.0-rc.2 | fails | fails | fails |
| v0.18.0-rc.1 | fails | fails | fails |
| v0.17.1 | fails | fails | fails |
| v0.16.1 | passes | fails | passes |

The `incomplete.cue` case, and both cases with an `else` clause, regressed in
339485ddf ("internal/core/adt: dependency-tracking comprehension pushdown",
v0.17.0-alpha.2): its parent 376710808 passes, it fails (checked by building both). The
fatal case without `else` already fails before that commit and on v0.16.1 (not bisected).

## Root cause

Confirmed with debug prints in an rc.3 build on both files. `comprehension.go:pushDownDeps`
registers the comprehension as a parent task of arc `x` (`arcState.parentTasks`), not a
task `#P` waits on. `unify.go:Vertex.lookup` runs it via `runReadyParentTasks` (:856); the
clauses fail and the error goes to the struct only: `processComprehension` returns an
incomplete error, which `sched.go:runTask` records with `t.node.addBottom` on `#P` (task
FAILED), and stores a fatal one with `n.node.state.addBottom` on `p` (:198, task SUCCESS).
The parent tasks are not in `x`'s counters, and `deferFieldSetKnown` skips a FAILED task as
done, so `x` finalizes with `errs == nil` and value `[...]`. The lookup returns `x` at its
`ArcMember` early return (:891), before both its `hasActiveParentTask` checks and the
`ArcPending` branch that reports failed parent tasks (:1084); hence `undefined field` without `x: [...]`.

## Proposed fix

graphops/cue branch `fix/rc3-comprehension-error-loss` (0c71b5024): in
`processComprehension`, when the clauses fail with a non-cycle error, record it also on
each arc the body and else clause declare (`addCompErr`, mirroring `pushDownDeps`'s walk,
recursing into nested field values, embedded structs and nested comprehensions). This is
the place where pushdown moved the dependency, so the error follows the same path. Cycle
errors are left to the arcs' own cycle handling. Golden changes: `eval/issue2550` (`foo`
now carries the guard's `undefined field` error), `comprehensions/pushdown` (one fewer
duplicate incomplete error), and `tools/flow` stats. Limit: only arcs that already exist
with an unfinalized state get the error (`getBareState` non-nil); dynamic fields and
pattern constraints in the body are not covered.

## Related

None found (searched "comprehension error lost", "if guard error", "comprehension
silently", "comprehension error not propagated", "pushdown comprehension error", and
variants).
