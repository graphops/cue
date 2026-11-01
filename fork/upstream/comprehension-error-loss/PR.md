# PR: comprehension-error-loss

Branch: cjorge-graphops/cue `fix/comprehension-error-fields`: commit d409d3585 plus fixup f9c6f0ec1, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`). Based on `cjorge/master` a0768bd98 (upstream master, v0.18.0-rc.3).

Before opening, replace `XXXX` with the issue number in the commit message and in: `cue/testdata/comprehensions/errorfields.txtar`, `internal/core/compile/testdata/sync/comprehensions/errorfields.txtar`.

## Title

```
internal/core/adt: fail the fields declared by a failing comprehension
```

## Body (plain text, as in the commit message)

```
Since https://cuelang.org/cl/1236923 ("internal/core/adt:
dependency-tracking comprehension pushdown"), a comprehension is a
parent task of the arcs for the fields its body and else clause declare,
rather than a task which the struct holding it must complete, so that
these arcs need not wait for the whole struct. When the clauses of the
comprehension fail, as when an if guard is an error or never becomes
concrete, processComprehension records the error on the struct only. An
arc which also has conjuncts from outside the comprehension, such as x
in

	out: p.x
	p: {
		x: [...]
		if b == 1 {
			x: [1]
		}
	}
	b: int

then completes as if the guard were false, and a reference to it which
is evaluated before the struct, such as out, yields [] without an error.
Before that commit, the evaluation of x included the comprehension, and
x failed with it. A guard which is an error, in a comprehension without
an else clause, has been ignored this way since
https://cuelang.org/cl/529524 ("internal/core/adt: use per-field
comprehensions").

Add the error as a conjunct to these arcs, walking the body and else
clause as pushDownDeps does. Being a conjunct, it is also seen where the
conjuncts of such an arc are used before the arc completes, as in an
embedding of p.x or a for loop over p. Cycle errors are left to the
lookups of the arcs, which report them.

In eval/issue2550, the guard of the comprehension which declares foo is
an error, so foo now fails with it. In comprehensions/pushdown, the two
comprehensions of cyclicError.a read the fields of each other, so the
second one now reports the error of the first one rather than one of
its own. The stats count the added conjuncts.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
