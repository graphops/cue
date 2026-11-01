# Prior art: pushdown-notify

**Verdict: file, referencing #4520/#4487 (and noting #4524 as different).** This is a new issue. No report, open or closed, covers a referring field that misses conjuncts a pushed-down comprehension adds to the referenced field after the comprehension's guard has resolved the reference. Every issue bisected to 339485ddf (cl/1236923) is closed, and each one's reproducer passes on rc.3 without our change. Our fix changes none of their results. #4520 and #4487, which the README cites as the same family, were fixed by other changes (b218faaa6, d47e4cee8/a63d3d1a1) that leave our case failing. The README says this now. Whether their mechanism matches ours is the README's analysis. I could not test it behaviourally, because their reproducers no longer fail.

The closest open issue is **#4524**: `m1: c1 & {y: 3}`, `c1: #B...`, `if c1.x == 2 {c1: #B}` under explicitopen, where `m1` stays open. It is not covered by our fix: it fails on rc.3 and on `cue-up-pushdown-notify-reference` with the same output. The plain-closedness analogue (`c1: {x: 2, ...}`, no explicitopen) passes on rc.3. With the guard reading the referrer instead (`if m1.x == 2 {c1: #B}`, `n4`), rc.3 accepts `m1.y` and our fix rejects it. So our bug can also show as lost closedness, but #4524 is a different bug. This is noted in the README.

Checked 2026-11-01. The corpus runs to 2026-10-30, and upstream/master is a0768bd98, which matches `git ls-remote`. A live `gh issue list` (created ≥ 10-29, and comprehension issues updated ≥ 10-30) shows no new issues.

## Candidates

The columns are v0.17.1 / rc3 / `cue-up-let-comprehension-pushdown` / `cue-up-pushdown-notify-reference`. The files are in `tmp/prior-pushdown-notify/c/` and `tmp/prior-let-pushdown/c/`.

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| own | repro.txtar | | | FAIL / FAIL / FAIL / PASS. Same with `CUE_DEBUG=sharing=0`, so sharing is not involved. |
| own variants | | | | `n4` (closedness, guard reads the referrer): FAIL / FAIL / FAIL / PASS. `v4376c` (`cp: test1 & {}`, `if cp.a == ... {test1: b: ...}`): FAIL / FAIL / FAIL / PASS. `n1` (guard reads `x` itself), `n3`, `n5`: PASS on all. |
| #4520 | disjunction default not applied when read in a comprehension condition | closed (b218faaa6) | RELATED (same commit, README's "same family") | FAIL / PASS / PASS / PASS |
| #4487 | spurious incomplete len error in an if guard over a comprehension-built list | closed (d47e4cee8, a63d3d1a1) | RELATED (same commit, pending-arc family) | FAIL / PASS / PASS / PASS. Its let variant is the let-pushdown topic, not this one: `v4487let` FAIL / FAIL / PASS / FAIL. |
| #4524 | field opened with `...` stays open when a comprehension later unifies it with the same definition | **open** | RELATED (similar shape, not fixed by ours) | FAIL on all four, with identical output for `m1`. Declared after the comprehension (`i4524b`): FAIL / PASS / PASS / PASS. |
| #4519 | definition referring to a field opened with `...` stays open | open | UNRELATED (explicitopen, no comprehension) | FAIL / PASS / PASS / PASS |
| #4376 | `let _x = self` misses fields produced by try/if/for | closed | RELATED (symptom: a reference misses comprehension fields; self-let mechanism) | Repro and `v4376a/b`: FAIL / PASS / PASS / PASS |
| #4422 | comprehension algorithm causes a hang (cites #4422 in the brief) | closed (1bf87f0f2) | UNRELATED (hang on a dynamic field under evaluation) | It terminates with the expected error on all four. |
| #4423 / #4425 | field not allowed / "field set was already referenced" (guard reads a field a comprehension adds to) | closed (3aaa50470, f2197d46e) | RELATED (guard-driven premature finalization; dropped resolver conjunct) | All PASS. Those fixes defer or requeue resolvers on the same node. Ours concerns a reference from another field. |
| #4533 | spurious cycle / dropped field via a let and inline unification | closed (e147bfc48) | RELATED (lookup forcing) | PASS on all |
| #4448 | spurious invalid interpolation | closed (d0182fcb6) | UNRELATED to this topic | FAIL / PASS / PASS / PASS |
| #3967 | evalv3 "undefined field" with cyclic conditionals | open | UNRELATED (pre-dates 339485ddf) | Fails on all with the same error |
| #3296 | incorrect default choice with a guard on `a` and `b: a \| *4` | open | UNRELATED (old, both evaluators) | Fails on all |
| #2555 | execution depends on file order | open | UNRELATED ("already iterated over by comprehension") | Same result on all four |
| #4535, #4526, #4532, #4392, #4440, #4474, #4478 | other comprehension/let issues since v0.17 | closed | UNRELATED | They pass on rc3. Our fix changes nothing in them. |

## Queries (corpus `cs.py`)

```
--all '339485dd'                 --all '1236923'
-i --all 'push.?down'
--all pushDownDeps | fieldConjunctsKnown | scheduleVertexConjuncts | addNotify | hasActiveParentTask
      | allAncestorsProcessed | insertConjunct
--all 'v0.17.0-alpha.2'          -i --all 'alpha\.2' 'comprehension'
-i --since 2026-05-01 --all 'notif'
-i --since 2026-05-01 --all '(missing|lacks|lack|without|does not (have|include|contain)).{0,40}field' 'comprehension|\bif\b'
-i --since 2026-05-01 --all 'reference' 'comprehension' 'missing|dropped|lost|not (see|include)'
-i --since 2026-05-01 --all 'silently' 'comprehension|if '
-i --since 2026-05-01 --all 'default' 'guard'
-i --since 2026-05-01 --all 'let' 'guard'
```

Also checked: every non-PR issue since 2026-05-20, triaged by title. Open evaluator issues with comprehensions and code were scanned, and I read the full threads of #4524, #4519, #4525, #4528, #4529, #4501, #4502, #4300, #3994, #3967, #3296 and #2555 (including its 2026 comments). I also read the commit messages of b218faaa6, d47e4cee8, e147bfc48, d0182fcb6, f2197d46e and 7d035b83b, and checked live `gh issue list`.
