# Prior art: comprehension-error-loss

Checked 2026-11-01 against the local corpus (issues/PRs/comments to 2026-10-30, discussions,
public Gerrit to ~2026-08, upstream git log; upstream/master re-fetched, still a0768bd98) and a
live `gh issue list` of everything created since 2026-10-28 (newest is #4541).

**Verdict: new issue.** No issue, discussion, CL or commit reports a field declared next to a
comprehension that is read without the error of the comprehension's guard, in either the
incomplete form (339485ddf) or the fatal form (748a68589, 2022). The closest upstream record is
the golden output of `cue/testdata/eval/issue2550.txtar`, which encodes the current behaviour
(see below). PR.md already mentions it.

## Candidates

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| eval/issue2550.txtar (test for #2550) | `foo: string @test(eq, string)` next to `if bar.missing == "x" {foo: ...}` | master test | SAME-CAUSE (encoded golden) | `out: foo` exports `incomplete value string` on v0.17.1 and rc.3, but `undefined field: missing` with cue-up-comprehension-error-fields. The test records `foo` as `string` without a TODO. It was written for a panic (#2550, 2023, closed), so nothing shows the maintainers chose this. Our PR changes this golden and says so. |
| #1846 | Silent failure on underdefined conditionals | closed 2022 (fixed by 2025) | RELATED | `cue eval -e result` printed nothing and exited 0 when a guard was non-concrete. That was the whole struct, which now reports the error. Our variant (`success: bool \| *false` next to the guard, `out: result.success`) errors on v0.17.1, rc.3 and the fix, because the struct is evaluated first. Not our path. |
| #3536 | cue cmd hides errors in comprehensions | closed (ab9dd3f8a, 2026-10-20) | RELATED | A failing `for` source in a command's tasks was hidden by "no tasks found". Fixed in cmd/cue reporting, not in the evaluator. |
| #4448 / d0182fcb6, #4533 / e147bfc48 | lookups of member arcs no longer force pushed-down comprehensions | closed | RELATED (same lookup path) | These touch `Vertex.lookup` for member arcs with pushed-down comprehensions. They are not the cause: d0182fcb6 is not in v0.17.1, which already fails, and the bisect points to 339485ddf. Neither commit mentions guard errors. |
| #4487 / d47e4cee8, a63d3d1a1; 154d2ce09 (else pushdown) | pending-arc settling, else-clause pushdown | closed / master | RELATED | Same pushDownDeps machinery. They do not touch the outcome of failed clauses. 154d2ce09 is in our base, which is why the `else` cases now behave like the body. |
| #4532 / 47d81c3cb | let built by an empty comprehension is "incomplete" when ranged over | closed | UNRELATED | The opposite symptom: a spurious error. |
| #2403, #2131, #1879 | comprehension-related silent results (2022-23) | closed | UNRELATED | Self-referential for comprehension / chained for / API iteration. |
| #4249 → #1730 | vet for comprehension bodies that can never unify | #1730 open (feature) | UNRELATED | Asks for static checks on bodies, not about guard errors. |
| #2713, #3996, #4535, #3992, #4029, #2542 | other comprehension/guard reports | closed | UNRELATED | Read, different mechanisms. |

## Queries (cs.py, --all, -i)
- `748a6858|529524|per-field comprehensions` (2022-23 threads on that change were checked by title, none about guard errors)
- `(error|bottom|conflict)...(in|of) (comprehension|if) (guard|condition|clause)...(ignored|lost|dropped|swallowed|not reported|hidden|silently)`
- `(guard|condition).{0,60}(ignored|swallowed|silently|not propagated|lost|dropped)` + `(comprehension|if)`
- `comprehension` + `error` + `(ignored|swallowed|silently|not propagated|not reported)` + `(guard|condition)`
- `--since 2022-06-01 comprehension` + `error` + `(lost|swallowed|disappear|vanish|ignored|masked|hidden)` + `(guard|condition|if)`
- `processComprehension`; `(failing|failed|erroneous) (comprehension|guard|if clause|condition)`; `ArcMember`; `parent task`; `failed task`; `pushDownDeps`; `1236923`; `339485dd`
- `non-concrete value \w+ in operand to ==` + `(if|comprehension)`; `(instead of|rather than) (an )?(error|incomplete)` + `if` + `(\[\]|\{\})`
- `cue vet -c` + `(succeeds|passes|exit 0|no error)` + `if`; `--since 2026-05-26` `(non-concrete|incomplete)` + `if` + success phrases; `--since 2026-05-26` `else` + error-loss phrases
- `(result|output|behaviou?r) (depends|differs) on (which|what|whether)` + `if`; evaluate/select-before + guard + error-loss phrases
- python scans: every issue since 2022-06 with an `if` guard plus "no error / should fail / silently / unexpected command success" in the body or comments (about 50 hits, all read by title, candidates read in full); titles with if/comprehension/guard plus pass/ignore/silent/incomplete/vet; discussions with an `if` guard plus error-loss phrases
- master testdata: `git grep` for eq:todo/incorrect markers in comprehensions/eval/cycle, and for comments on guard errors that are ignored or not propagated
- `gh issue list --search created:>=2026-10-28`

Reproducers run (in `~/.claude/jobs/10b601ab/tmp/prior-comprehension-error-loss/`): v1846.cue and
v2550.cue with v0.17.1, rc.3 and cue-up-comprehension-error-fields. Our repro.txtar fails on rc.3
and passes on the fix.
