Title: evaluator: field added by a comprehension after its guard read a disjunction-unified reference is silently lost (v0.17.0 regression)

> **Report only.** No fix is proposed: a fix appears to need a design
> decision on disjunctions (see Analysis). File it alongside
> share-disjunction-safe, whose fix turns one shape of this bug back from a
> cycle error into this silent loss.

### What version of CUE are you using (`cue version`)?

<pre>
$ cue version
cue version v0.18.0-rc.3

CUE language version v0.18.0
</pre>

Also reproduced with the current master.

### Does this issue reproduce with the latest stable release?

Yes. v0.17.1 gives the same wrong result. v0.16.1 gives the expected one.

### What did you do?

```
exec cue export x.cue
cmp stdout want

-- x.cue --
x: {
	args: a: 1
	if out.a == 1 {args: b: 2}
}
out: x.args & ({...} | int)
-- want --
{
    "x": {
        "args": {
            "a": 1,
            "b": 2
        }
    },
    "out": {
        "a": 1,
        "b": 2
    }
}
```

### What did you expect to see?

A passing test. `out` is `x.args` unified with `{...} | int`, so it has every
field of `x.args`, including `b`. v0.16.1 prints that.

### What did you see instead?

`out` silently lacks `b`, with exit status 0:

```
    "out": {
        "a": 1
    }
```

### Analysis

Bisected to 339485ddf ("internal/core/adt: dependency-tracking comprehension
pushdown"). Its parent gives the expected result.

The guard of the comprehension in `x` reads `out.a`. That evaluates `out`,
and with it the disjunction `{...} | int`, which is decided at that point,
before the comprehension has added `b: 2` to `x.args`. Once the comprehension
adds `b`, the new conjunct appears to reach the original `out` but not the
disjunct that was chosen for it, so the result lacks `b`. The upstream
pushdown-notify fix, which makes such a reference register for later
conjuncts, does not change the result, which is consistent with this.

Before 339485ddf, the comprehension was part of the evaluation of `x.args`,
so `x.args` was complete before `out` copied it. With pushdown, the
comprehension is a parent task of the arc it adds to, and `out` can be decided
while that task is still running.

A fix seems to require re-evaluating, or re-opening, a disjunction that has
already been decided when a pending comprehension adds conjuncts to a value
it was built from. Alternatively, it could delay deciding such a disjunction
until the parent tasks of what it reads are done, which here is the very
comprehension waiting on it. Either way this is a design question, so this
report does not propose one.

The related case below, where the comprehension sits in the struct being
copied and is lost when the struct is cloned for a disjunct, has a separate,
local fix (prepared as "keep the parent tasks of an arc cloned for a
disjunct"). That fix does not change this case.

```
#M: {...}
x: {
	if true {a: 1}
	for k, v in out {b: v}
}
out: x
out: #M | int
```

### Related

- 082c52c57 ("finalize a shared forward reference before a disjunction") turns
  the variant `#A: #S | int`, `out: x.args & #A` into a spurious `x.args:
  cyclic reference to field b`. A fix for that regression restores the silent
  loss reported here.
- #4487, #4520: other bugs from the same pushdown change, already fixed. Their
  reproducers are unaffected.
