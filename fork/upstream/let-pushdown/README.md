Title: evaluator: conjunct silently dropped when a comprehension body declares a let (v0.17.0-alpha.2 regression)

### What version of CUE are you using (`cue version`)?

<pre>
$ cue version
cue version (devel)

CUE language version v0.18.0

Go version go1.26.0
          GOARCH amd64
            GOOS linux
</pre>

Built from master at a0768bd98 (v0.18.0-rc.3). The v0.18.0-rc.3, rc.2 and rc.1 releases behave the same.

### Does this issue reproduce with the latest stable release?

Yes. It fails on v0.17.1 and passes on v0.16.1. It first fails in v0.17.0-alpha.2.

### What did you do?

```
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

### What did you expect to see?

A passing test, as with v0.16.1. `x.n` is 1, so the top-level comprehension adds `on: true`, `x.on` is `true`, and the `if on` body adds `out: 1`.

With `out: 1` in place of the let and `out: v`, the test passes on master.

### What did you see instead?

`cue export` exits 0. The `on: true` conjunct is dropped without an error:

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

### Analysis

Bisected to 339485ddf ("dependency-tracking comprehension pushdown"); its parent 376710808 passes. At 339485ddf the variant without the let fails too. e147bfc48 ("stop lookups from forcing pushed-down comprehensions") fixed that variant but not this one.

`pushDownDeps` (internal/core/adt/comprehension.go) adds `fieldConjunctsKnown` to a comprehension's `completes` for each let in its body. So the `if on` comprehension is not counted as pushed down. To resolve `x.n` in the top-level guard, `SelectorExpr.resolve` calls `OpContext.processRef`, which runs `process(arcTypeKnown|fieldSetKnown, yield)` on `x`. That selects tasks that complete `fieldConjunctsKnown`, so the `if on` comprehension runs now. Its guard reads `x.on` while the top-level comprehension that adds `on: true` is still evaluating its own guard further up the stack, and `x.on` is finalized at its default. `Vertex.insertConjunct` (fields.go) later finds `x.on` finalized and drops `on: true` without an error. I confirmed this sequence with debug prints.

This is one instance of a problem that dates from 339485ddf: a field is finalized while a pushed-down comprehension registered on it (here, the top-level one) is still running and can add to it. The same silent drop still happens on master when the `if on` body has a dynamic field (`("out"): 1`), a pattern constraint (`[string]: int`), `...`, or an embedded reference (`let v = {out: 1}` followed by `v`) instead. Those bodies can declare fields that are not known in advance, so they are not pushed down for a good reason. A general fix would need to detect this re-entrancy and report an error or retry, instead of finalizing the field. That looks like a larger change. Turning every conjunct dropped on a finalized arc into an error is not enough on its own: in a scratch build, passing tests drop conjuncts this way 28 times across 7 test cases, for example in `cycle/compbottom2.txtar`.

### Proposed approach

A let in a comprehension body can only be referred to from within that body. The existing comment in that case gives this as the reason not to track the let's dependencies. For the same reason, the let's arc is never reached through iteration or lookup of the enclosing struct, as 3d1cf46d6 (#4478) noted when it allowed let arcs in a frozen field set. I propose that `pushDownDeps` stop adding `fieldConjunctsKnown` for a `*LetField`. The change removes one line. Comprehensions whose body has only fields and lets are then pushed down, like those whose body has only fields.

The change also affects `scheduler.mayDeclareArcs`: it no longer treats such a comprehension as one that may declare new fields. In `comprehensions/pushdown.txtar`, `letCycleSelfKeyword2` then reports `undefined field: hosts` instead of `cannot range over s.hosts (incomplete)`, which matches the same test without the let. e147bfc48 made the same change to the `issue3990` golden in `nested2.txtar`. No other test output changes, including the eval stats.

This is the narrow fix for the let case. It does not address the general problem described above.

Related: #4448. With a let added to the body of the guarded comprehension in `issue4448.txtar`, that test fails again because the `"\(port)"` interpolation stays unresolved. The proposed change fixes that too. #4520, a different symptom, was also bisected to 339485ddf. #4478 also involves a let in a comprehension, but is a different bug. Its test still passes with the proposed change.
