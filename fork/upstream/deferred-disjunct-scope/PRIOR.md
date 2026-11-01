# Prior art: deferred-disjunct-scope

**Verdict: file, referencing #4510/#4532.** This is a new issue. Nothing reports an error of a let or inline struct escaping its disjunct, or sticking to the wrong one, after b68cd7163. The single regression reported against b68cd7163 (#4532) was fixed by 47d81c3cb, which is in rc.3, and our reproducer still fails on rc.3.

Checked 2026-11-01. The corpus covers up to 2026-10-30. A live `gh issue list` (updated ≥ 10-30) shows nothing newer that is relevant. upstream/master = a0768bd98 = v0.18.0-rc.3.

## Candidates

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| #4510 | a used scalar of an inline struct or let is not checked against its `!=`, `=~` or builtin validators | closed (b68cd7163) | RELATED (the issue whose fix introduced ours) | Its thread has only the "Closed by" comment, and nobody reported a regression on it. Its repro fails on rc.1 and passes on rc.2, rc.3 and with our fix, so the fix keeps #4510 fixed. |
| #4532 | let built by a comprehension that matches nothing is "incomplete" when ranged over (rc.2 regression) | closed (47d81c3cb) | RELATED (same introducing commit, different mechanism) | That bug was a pending completion that kept a let from being marked as a struct. Ours is the completion being recorded on a task from outside the disjunct. The repros (both the issue's and mvdan's): rc.1 PASS, rc.2 FAIL, rc.3 PASS, our fix PASS. Our repro fails on rc.3. README already cites 47d81c3cb (#4532) for the guard-bypass alternative. |
| b68cd7163 commit message: "inline struct that refers back to a field of the disjunction it is read from … evaluated once, against the first disjunct" | (no issue) | known, untracked | RELATED, not ours | That is upstream's acknowledged pre-existing bug: the inline struct reads back into the disjunction. In ours, the let (b.cue) or the inline struct (a.cue) does not read the referring disjunction. No issue was filed for it. |
| #4533 | spurious "cycle with field" / dropped field reading a let's field through an inline unification | closed (e147bfc48) | UNRELATED (from d0182fcb6, pushdown/lookup) | It has no disjunction. It mentions b68cd7163 only to cite #4532. |
| #4531 | disjuncts ruled out by a concrete discriminator still evaluate their recursive bodies | open | UNRELATED (performance, #2851) | |
| #4522 / #4523 | "field not allowed" regressions | closed (c4a272fd2) | UNRELATED (closedness, from c172d9cbd) | |
| #4519 | definition referring to a field opened with `...` stays open | open | UNRELATED (explicitopen) | It matched only on "incomplete value {…} \| {…}". |

Our reproducer: v0.17.1 and rc.1 fail only on `b.cue` (the old incomplete error, as the README says). rc.2 and rc.3 fail on `a.cue`. `cue-up-deferred-completion-disjunct-scope` passes. The candidate txtars and logs are in `prior-deferred-disjunct-scope/`.

## Queries (corpus `cs.py`)

```
-i --all 'b68cd71'
-i --all '#4510|issue/4510|issues/4510'
-i --all 'complete nodes read through a let'
-i --all 'rootedTaskNode|handleDeferred|evalStateCI|pushOverlay'
-i --all 'deferred completion'
-i --all 'the first disjunct|first one.s value'
-i --since 2026-09-01 --all '\blet\b' 'disjunct'
-i --since 2026-09-01 --all 'inline (struct|unification)' 'disjunct'
-i --all '(escap|leak|surviv|wrong) (the )?disjunct|error (of|from) a discarded|discarded (arm|disjunct)|eliminate(d|s)? the wrong'
-i --since 2026-10-15 --all 'field not allowed' 'disjunct'
-i --since 2026-06-01 --all 'every disjunct|each disjunct' 'inline|let'
-i --since 2026-10-01 --all 'validator' 'let' 'disjunct'
-i --since 2026-06-01 --all 'wrong disjunct|invalid disjunct|disjunct (is|was) (kept|accepted|chosen)'
-i --since 2026-10-15 --all 'incomplete value \{'
-i --since 2026-10-20 --all 'let' 'disjunct|\|' 'regression'
```

Also checked: the full threads of #4510, #4531, #4532, #4533, #4522 and #4523; the messages of b68cd7163, 47d81c3cb and 33f485845, and the b68cd7163 nonrooted.txtar diff; live `gh issue list`/`gh pr list`.
