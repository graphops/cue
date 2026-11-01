# Prior art: try-finalize

Checked 2026-11-01 against the local corpus (issues/PRs/comments to 2026-10-30, discussions,
public Gerrit to ~2026-08, upstream git log; upstream/master re-fetched, still a0768bd98) and a
live `gh issue list` of everything created since 2026-10-28 (newest is #4541).

**Verdict: new issue.** Reference #4485 and #4486 as different `try` bugs (already in README).
Nothing upstream reports, tracks or fixes a try clause whose binding expression refers to the
fields its own body yields into.

## Candidates

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| #4485 / D4482 | try { x: b: x.a? } | closed (5e2823acf) | RELATED | Body form, a `?` lookup shadowed by the body's own field in pre-evaluation. Its repro passes on rc.3 and on our fix (fails on v0.17.1). Our shape needs the binding form: `try { x: a: <=2, _y: x? }` and `try { x: b: 1, _y: x? }` are already correct on v0.17.1, rc.3 and the fix. |
| #4486 | nested try reports cyclic reference to field claim | open, mvdan, NeedsInvestigation, 0 comments | RELATED, not SAME-CAUSE | Repro fails on rc.3 and still fails on cue-up-try-own-arcs-finalize; passes v0.17.1. Different path (nested try, placeholder arc type frozen). |
| #4483 | cannot reference optional field inside a try body; panic with a binding | closed (e7fa93e0c, f26dac4b2, 6e0c5c8ac) | RELATED | Pre-evaluation deferring the body's own tasks. e7fa93e0c says rooted vertices the body references "are finalized for good". Our fix only exempts arcs the try comprehension is a parent task of, so it stays consistent with that (tryRootedCycle* tests are unchanged). |
| #4098 (posch's 2026-08-18 try comment) | eval: a case where order matters? | closed (7d035b83b) | RELATED | 7d035b83b part 3: a try pre-evaluation froze rooted vertices' schedulers. The try comment's repro gives the right result in both orders on rc.3 and on the fix (v0.17.1 was wrong in one order). |
| #4423 / #4425 | field not allowed / "field set was already referenced" (339485ddf) | closed | RELATED (same pattern, `if` form) | The `if` repro of #4425 passes everywhere. Its try analogue `try L = local? { local: _zones[L.zone] }` drops `sos_region` silently on v0.17.1 and rc.3, and is correct with our fix. This is our bug in another form, not reported anywhere. |
| try.txtar `shadowing.{letBase,forBase,sharedBase}` | known-incorrect (`eq:todo`) upstream tests | master | UNRELATED | Same outputs on rc.3 and the fix. Pre-evaluation lookup shadowing, not finalization. |
| #4347, #4348, #4376, #4421, #4490, #4271, #4537 | other try issues | closed/open | UNRELATED | Optional refs in nested try / lists / let self / perf / error builtin panic / fmt / trim. |
| D4019 (try proposal) + D4386 | proposal threads | - | UNRELATED | No discussion of a binding expression referring to the body's target. |
| 7100de2be | test function identity (try known-wrong gaps) | master | UNRELATED | `func` admission, not try evaluation. |

## Queries (cs.py, -i unless noted)
- `\btry\b` + `(@experiment\(try|try clause|try comprehension|try [A-Z]\w* *=|try \{|\?-marked|\bx\?)` (all hits read by title, candidates read in full)
- python scan of all issues and comments matching the try experiment/clause syntax (22 issues, listed above)
- `cyclic reference to field` + `\btry\b`; `TryClause`; `inline vertex`; `finalizeSharing`; `insertConjunct`; `pushDownDeps`; `1236923`; `339485dd`; `dependency.tracking comprehension`
- `\btry\b` + `(silently|lost|ignored|dropped|disappear|missing)` + try syntax; `\btry\b` + `out of bound` + `@experiment`; `\btry\b` + `field set was already referenced`; `try [A-Za-z_]\w* *= *[\w.]*\?`
- case-sensitive `skipTry|tryBody|isTryBody|TryClause\.yield`; `pre-?evaluat` + `try`
- Gerrit try CLs (1221644, 1231444, 1231443, 1236766, 1236767) grepped for cycle/finalize
- master testdata: `git grep` of try txtar files for `todo`/`incorrect`/`cycl`, all try bodies and bindings in try.txtar
- `gh issue list --search created:>=2026-10-28`; `gh issue view 4486 --comments`

Reproducers run (in `~/.claude/jobs/10b601ab/tmp/prior-try-finalize/`): i4485, bind4485, i4486,
p4098 (both orders), i4425/i4425try/i4425try2, todo.cue (shadowing todos), nb/nb2 (body form), with
v0.17.1, rc.3 and cue-up-try-own-arcs-finalize. Our repro.txtar: fails on rc.3, passes on the fix.
