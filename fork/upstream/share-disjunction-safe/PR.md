# PR: share-disjunction-safe

Branch: `fix/share-disjunction-in-progress` (commit 6c5e9900c), based on `cjorge/master` (v0.18.0-rc.3).
Replace `XXXX` with the issue number in both the commit message and `cue/testdata/eval/sharing.txtar` (and its sync copy under `internal/core/compile/testdata/sync/eval/`) before opening.

## Title

```
internal/core/adt: only finalize a shared vertex early when finalizing the field
```

## Body (plain text, as in the commit message)

```
Since https://cuelang.org/cl/1255872, a field sharing a reference
finalizes the referenced vertex when it meets a disjunction with a
reference as a disjunct, so that the field can keep sharing. The field
may be evaluated only in part, though, such as for a selection made
from within an evaluation the vertex depends on. Finalizing the vertex
then forced that evaluation before it was done: a comprehension of the
vertex reading the field was frozen before it added its fields, which
were reported as cyclic references. For example,

	#S: {...}
	#A: #S | int
	x: {
		a: 1
		if out.a == 1 {b: 2}
	}
	out: x & #A

failed with "x: cyclic reference to field b", where both x and out
were {a: 1, b: 2} before that change.

Finalize the vertex early only while the field is itself being
finalized, when finalizeSharing would finalize the vertex anyway once
the field is complete. Otherwise the field keeps sharing only a vertex
that is final already, as before that change. The condition is on the
field rather than on the vertex: the evaluation in progress may belong
to any vertex the shared one reaches, such as one it refers to, so the
vertex and its fields may all be idle.

The stats of issue4512.txtar, which that change improved, are
unchanged, as are those of every other test but the one extended here.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```
