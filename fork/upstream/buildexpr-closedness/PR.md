# PR: buildexpr-closedness

Branch: cjorge-graphops/cue `fix/buildexpr-finalize-context`: commit a72a7f272 plus fixup a7d837b97, to squash into it at filing time (`git rebase -i --autosquash cjorge/master`). Based on `cjorge/master` a0768bd98 (upstream master, v0.18.0-rc.3).

Before opening, replace `XXXX` with the issue number in the commit message and in: `cmd/cue/cmd/testdata/script/eval_e.txtar`, `cue/build_test.go`.

## Title

```
cue: finalize BuildExpr values in the context that evaluated them
```

## Body (plain text, as in the commit message)

```
Since https://cuelang.org/cl/1238118, which fixed #4366, Vertex.unify
skips the typo check when a vertex is finalized by an OpContext other
than the one which started evaluating it, as the original evaluation
runs the check when its own Finalize unwinds.

Context.BuildExpr never let that happen. It evaluates the expression
with adt.Resolve, which evaluates a conjunction or a struct literal only
partially, leaving its arcs unprocessed, and then hands the vertex to
Context.make, which finalizes it with a new OpContext. The typo check
was thus skipped, and a definition accepted any field in an expression
built by BuildExpr. For example, with

	#S: {a?: int}
	d: {b: 1}

"cue eval -e '#S & d'" printed "b: 1" rather than reporting that b is
not allowed, as v0.16.1 did and as the same conjunction in a file does.
Nested forms such as "{y: #S & d}" were affected in the same way.

Finalize the vertex with the OpContext which evaluated it, as
Context.Encode already does, so that Context.make has nothing left to
do. The other Context and Value methods which evaluate an expression,
such as Value.FillPath and Value.Unify, already finalize the result in
the same OpContext.

Fixes #XXXX

Signed-off-by: Carlos Jorge <carlos@graphops.xyz>
Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>
```

## At filing time

The fixup (a7d837b97) adds a Go API test, `TestBuildExprClosedness` in
cue/build_test.go, next to the eval_e.txtar cases. The commit message does not
describe the tests and stays as is; the README's "plus a case in
cmd/cue/cmd/testdata/script/eval_e.txtar" can read "plus cases in
cmd/cue/cmd/testdata/script/eval_e.txtar and cue/build_test.go".
