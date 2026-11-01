# Prior art: disjunct-pushdown-notify

**Verdict: new issue, referencing #4487 (and #4520 as the same family).** No issue, comment, discussion, public Gerrit change or upstream commit covers comprehension-declared fields lost, or a cycle missed, in the disjuncts of a field unified with a disjunction. d47e4cee8 (the introducing commit) says "Updates #4487". #4487 was closed on 2026-10-01 by a63d3d1a1 and has no comments (checked live). Its reproducer passes on rc.3 and with our change. Nothing in the upstream log after d47e4cee8 touches `parentTasks` in overlay.go (`cloneScheduler` has never copied them).

Checked 2026-11-01. The corpus runs to 2026-10-30. `git ls-remote upstream master` = a0768bd98 (= v0.18.0-rc.3). A live `gh issue list --search created:>=2026-10-29` shows #4537 to #4541, none related.

## Candidates

Columns: v0.17.1 / rc.3 / `cue-up-disjunct-pushdown-notify`. Files are in `tmp/prior-disjunct-pushdown-notify/`.

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| own | repro.txtar (`out: x & ({...} \| int)`, the comprehension in `x` iterating `out`) | | | PASS / FAIL / PASS. Also FAIL on rc.1 and rc.2. d47e4cee8^ PASS, d47e4cee8 FAIL. |
| own `self.cue` | the comprehension inside the disjunction's operand | | | PASS / FAIL / PASS |
| own `guard.cue` | `if out.b == _\|_ {b: 1}` | | | cycle error / `{a: 1}` / cycle error. rc.2 also reports a cycle. Bisected to e147bfc48 (cl/1257111). |
| #4487 | spurious incomplete len error in an if guard over a comprehension-built list | closed (a63d3d1a1; d47e4cee8 "Updates") | RELATED (introducing commit's issue, pending-arc family) | `i4487.cue`: FAIL / PASS / PASS |
| #4520 | disjunction default not applied when read in a comprehension condition | closed (b218faaa6) | RELATED (guard plus disjunction, 339485ddf family, different mechanism) | `i4520.cue`: FAIL / PASS / PASS |
| #4533 | spurious "cycle with field", or a dropped field, reading a let's field through an inline unification | closed (e147bfc48) | UNRELATED (lookup forcing, no disjunction of the referrer) | Not affected (closed, its test is in the suite, which passes) |
| #4428 | `fix --exp=aliasv2` breaks code (`for k, v in this.#data` next to `string \| *"Opaque"`) | closed | UNRELATED (the disjunction is on a sibling field, aliasv2 rewrite) | |
| #3524 | recursive definition with disjunction gives a structural cycle | open | UNRELATED (structural cycle cut-off) | |
| #3967 | evalv3 "undefined field" with cyclic conditionals through a struct disjunction | open | UNRELATED (pre-dates 339485ddf, genuinely cyclic) | |
| pushdown-notify (our draft) | a referring field misses fields a comprehension adds later | not filed | RELATED. Its disjunction variant (`a3.cue`, `out: x.args & ({...} \| int)`) is NOT fixed by this change, nor by the pushdown-notify fix. | `a3.cue`: FAIL / FAIL / FAIL. Passes at 339485ddf^. |

## Queries (corpus `cs.py`)

```
-i 'parentTasks'                       -i 'clone' 'parent ?task'
-i 'overlay' 'pushdown|pushed.down'     -i 'disjunct' 'pushed.down|pushdown'
-i 'cloneScheduler|cloneRoot|cloneTask'
-i 'settle a pending arc'               -i 'd47e4cee8|I5371838988a1790ce2273f6a0e537a977879af66|d0448fd334ddedc63b3081f3d3ba14fea2372eeb'
-i '#4487|issue/4487'                   (and all comments on #4487 and #4520 in comments.jsonl)
--since 2026-09-15 -i 'rc\.1|alpha\.3' 'comprehension'
--since 2026-05-01 -i 'silently' 'comprehension' 'disjunct'
--since 2026-05-01 -i 'disjunction' 'for .* in|comprehension' 'missing|lost|dropped|absent|not found|lacks'
--since 2026-06-01 -i 'comprehension' 'disjunct' 'iterat|for k, v in|for _, v in'
--since 2026-05-01 -i 'for [a-z_]+, [a-z_]+ in (out|self|this|X)\b'
--since 2026-06-01 -i 'pending arc'     --since 2026-08-01 -i 'empty disjunction' 'comprehension'
--since 2026-05-01 -i 'field.*(missing|dropped|lost)' 'disjunction'
--since 2026-05-01 -i '\{\.\.\.\} *\| *int'
```

I also triaged every issue created since 2026-09-01 by title (#4476 to #4541), and read the bodies of #4487, #4520, #4526 and #4533 and the recent comment on #3524.
