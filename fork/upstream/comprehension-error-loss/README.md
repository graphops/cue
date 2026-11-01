evaluator: a field declared next to a comprehension ignores the error of the comprehension's guard

### What version of CUE are you using (`cue version`)?

v0.18.0-rc.3 (also master at a0768bd98).

### Does this issue reproduce with the latest stable release?

Yes, v0.17.1 behaves the same. The `incomplete.cue` case below, and the same
cases with an `else` clause, are a regression: v0.16.1 reports the error. The
`fatal.cue` case without an `else` clause has been wrong since v0.5.0.

### What did you do?

```
# The guard never becomes concrete.
! exec cue export incomplete.cue
! stdout .
stderr 'non-concrete value int in operand to =='

# The guard is an error.
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

### What did you expect to see?

Both commands fail. The value of `x` depends on a guard which cannot be
decided, so `out` is incomplete in `incomplete.cue`, and it fails with the
error of `b` in `fatal.cue`. v0.16.1 reports
`#P: non-concrete value int in operand to ==` for `incomplete.cue`.

### What did you see instead?

```
> ! exec cue export incomplete.cue
[stdout]
{
    "out": []
}
FAIL: repro.txtar:2: unexpected command success
```

and `cue export -e out fatal.cue` prints `[]` and exits 0. `x` is evaluated as
if the guard were false. This needs `out` to be evaluated before the struct
holding `x`: once that struct is evaluated, it carries the error and selecting
from it fails. The same happens when `p.x` is embedded (`out: {p.x}`), when
ranging over `p`, and through the Go API (`LookupPath("p.x")`), and with an
`else` clause. `cue vet -c incomplete.cue` also exits 0.

### Analysis

`incomplete.cue`, and the cases with `else`, bisect to 339485ddf
("internal/core/adt: dependency-tracking comprehension pushdown"); its parent
376710808 reports the error. Since that change, `pushDownDeps` makes a
comprehension a parent task of the arcs for the fields its body declares, so
that they need not wait for the enclosing struct. When the clauses fail,
`processComprehension` records the error on the struct only (as a failed task
for an incomplete error, with `addBottom` for a fatal one). `x` has its own
conjunct `[...]`, completes without the comprehension's contribution, and
`Vertex.lookup` returns it at its `ArcMember` early return. Before 339485ddf the
comprehension was evaluated as part of `x`, so `x` failed with it.
`fatal.cue` without `else` bisects to 748a68589 ("internal/core/adt: use
per-field comprehensions", v0.5.0-alpha.1).

This is one instance of a cause shared with other regressions from 339485ddf:
a field completes without the outcome of a pushed-down comprehension that
targets it.

### Proposed approach

When the clauses of a comprehension fail (other than with a cycle error, which
the lookups of these arcs already report), add the error as a conjunct to the
arcs for the fields the body and `else` clause declare, walking them as
`pushDownDeps` does. As a conjunct, the error also reaches references that use
the arc's conjuncts before it completes, such as an embedding of `p.x` or a
for loop over `p`, which recording it in the arc's state would not.
Alternatively, `Vertex.lookup` could report a failed parent task for a member
arc too, as it does for a pending one, but that misses a guard that fails only
after the lookup, and the arc's own value as seen through the API. A fix along
the first lines is ready (+42 lines in `internal/core/adt/comprehension.go`,
plus tests).
