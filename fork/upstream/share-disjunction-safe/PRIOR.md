# Prior art: share-disjunction-safe

**Verdict: file, referencing #4512/#4530.** This is a new issue. No issue, comment, discussion, CL or
commit reports a spurious "cyclic reference to field" (or the related export panic) caused by
082c52c57. #4530 is the only other report against 082c52c57. It was fixed by 4232b8d36, which is in
rc.3, and our reproducer still fails on rc.3.

Checked 2026-11-01. The corpus covers up to 2026-10-30. The live `gh issue list` (updated ≥ 10-30)
shows nothing newer that is relevant. upstream/master is still a0768bd98 = v0.18.0-rc.3.

## Candidates

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| #4530 | a disjunction's default survives unification with a forward reference that excludes it (rc.2 regression) | closed (4232b8d36, 10-29) | RELATED (same introducing commit, different mechanism) | Bisected to 082c52c57 too. The cause there is cycle detection dereferencing the recorded reference arc, so a conjunct is dropped. Ours is the early `Finalize` freezing a running comprehension. Both of its reproducers (issue and mvdan's comment): rc.2 FAIL, rc.3 PASS, our fix PASS. Our repro: rc.3 FAIL. The 19d6c62c0 tests (Updates #4530) cover dropped conjuncts, not frozen comprehensions. |
| #4512 | a reference unified with a definition type is re-evaluated per reference path | closed (1d5bdcf57) | RELATED (the issue 082c52c57 "Updates") | It is a performance issue. Its thread has only the "Closed by" comment, and nobody reported a regression on it. Per README, our fix leaves the issue4512.txtar stats unchanged. |
| #4540 | typed dependency chains grow exponentially with an inline definition union | open | UNRELATED (perf follow-up of #4512) | Our fix leaves its counts unchanged: 34,459 unifications (inline) and 55 (named) on both rc.3 and the fix. |
| #4520 | disjunction default not applied when read in a comprehension condition before the disjunction is declared | closed (b218faaa6) | RELATED (guard selecting through a shared vertex) | The symptom differs ("undefined field"), and it is from 339485ddf. Repro: rc.2 FAIL, rc.3 PASS, fix PASS. The tests in e798b9a41 cover forward-declared disjunctions, not `out: x & #A` read from inside `x`. |
| #4526 | default lost when a field unified with a reference to its own definition is read earlier (rc.2 regression) | closed (58a3543f7) | UNRELATED (from 1c287ec21) | Repro (module at v0.17.1): rc.2 FAIL, rc.3 PASS, fix PASS. |
| #4533 | spurious "cycle with field", or a dropped field, reading a let's field through an inline unification | closed (e147bfc48) | UNRELATED (from d0182fcb6, sharing-independent) | Both of mvdan's reproducers: rc.2 FAIL, rc.3 PASS, fix PASS. |
| #4532 | let built by a comprehension that matches nothing is "incomplete" | closed (47d81c3cb) | UNRELATED (from b68cd7163) | Its report names 082c52c57 only to say it is distinct. |
| upstream tests (sharing.txtar, issue4512/4526/4520, explicitopen) | | master | not covered | No test or TODO has a comprehension reading a field that shares its own struct through a disjunction. |

Our reproducer and variants: v0.17.1 and rc.1 PASS, rc.2 and rc.3 FAIL, `cue-up-share-disjunction-in-progress` PASS. The `y: {…}; x: y` indirection and the embedded-expression variant from the README behave the same way. `prior-share-disjunction-safe/` holds the candidate txtars and logs.

## Queries (corpus `cs.py`)

```
-i --since 2026-09-01 'cyclic reference'
-i 'cyclic reference to field'
-i --since 2026-10-15 'cycl'
-i --all '082c52c'
-i --all '1255872'
-i --all 'sharesSingleValue'
-i --all 'finalizeSharing'
-i --all 'finalize a shared forward reference'
-i --all 'shared forward reference'
-i --all 'forward reference.*disjunction'
-i --all 'rc\.?2 regression'
-i --all 'v0\.18\.0-rc\.[23]'
-i --since 2026-06-01 --all 'not properly initialized' / 'updateArcType' / 'reportFieldCycleError' / 'completeAllArcs'
-i --since 2026-10-20 --all 'shar' 'comprehension|guard' 'disjunct'
-i --since 2026-10-20 --all 'sharing=(0|false)'
-i --all 'properly initialized'
-i --since 2026-10-15 --all 'panic'
```

Also checked: the full threads of #4512, #4530, #4520, #4526, #4532, #4533 and #4540; the upstream log
v0.18.0-rc.1..master for sharing commits (fd3117d83, 344a06b62, d3bb17ed9, d323d73f0, 19d6c62c0,
e798b9a41, b218faaa6, 4232b8d36); `git grep` of upstream sharing tests; live `gh issue list`/`gh pr list`
(updated ≥ 2026-10-25/30); GitHub Discussions (latest updated).

**Interaction with a side finding (verified 2026-11-01).** With the fix,
`out: x.args & #A` (the comprehension adds to `x.args`, the field `out`
selects) returns to the v0.17.1/rc.1 result: `out` silently lacks `b`. rc.2
and rc.3 report a spurious cycle there instead. That loss is the older bug
since 339485ddf (`_side-findings`, "x.args"). The README states this as a
caveat, so that the x.args loss is reported alongside, not discovered in
review.
