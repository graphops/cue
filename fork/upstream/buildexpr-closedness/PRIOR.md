# Prior art: buildexpr-closedness

**Verdict: new issue.** Its analysis cites #4366 (fixed by a9f3f2a09, the introducing commit) and #2680. No issue, comment, discussion, CL or commit reports that `-e`/`--expression` or `Context.BuildExpr` accepts fields a definition forbids. Nobody reported a regression on #4366, whose thread has only a "cc" comment, or on cl/1238118. No later commit touches the a9f3f2a09 skip or the finalize step in `BuildExpr`. The bug still reproduces on upstream/master a0768bd98 (= rc.3).

Checked 2026-11-01. The corpus runs to 2026-10-30, and upstream/master is a0768bd98. Live `gh issue list` (updated ≥ 2026-10-30) and `gh search issues BuildExpr` (updated ≥ 2026-06-01) found nothing newer that is relevant.

## Candidates

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| own | repro.txtar | | | rc.3 FAIL (line 6, `-e '#S & d'` succeeds), `cue-up-buildexpr-finalize-context` PASS. |
| #4366 / cl/1238118 / a9f3f2a09 | json.Marshal(this) self-cycle breaks disjunction default resolution | closed | RELATED (the fix that introduced ours) | The thread has only "cc @nxcc". No comment, CL or commit mentions a closedness loss caused by the skip. The README already cites it. |
| #2680 | cue/ast: Context.BuildExpr cannot create closed struct | closed (2c281cb56, v0.17.0-alpha.1) | RELATED (same API and symptom, different mechanism) | Its Go program unifies the built value afterwards with `Value.Unify`, and `TestBuildExprClose` keeps that passing. The CLI form of the same idea, `-e 'close({}) & w'` with `w: a: 1`, is broken again by our bug: v0.16.1 reports `a: field not allowed`, v0.17.1 and rc.3 print `a: 1`, and the fix rejects it. The README now notes this. |
| CL1238753 | internal/core/adt: guard parent conjuncts by OpContext in typo check | abandoned (2026-09-17, "not needed anymore") | UNRELATED (same area) | It fixes an index-out-of-range panic in `hasParentEllipsis` when a value is re-rooted under a fresh OpContext (yaml.Marshal, #2782). It does not touch the skipped typo check. |
| #4349 | cmd/cue: -e flag seems to disregard recursive closedness with jsonschema output | closed (2b2987e31) | UNRELATED | It concerns the JSON Schema generator (`def --out jsonschema`), not evaluation. |
| #3930 | evaluator: cue export -e evaluates fields that are out of scope | open | UNRELATED | Performance, and errors outside the `-e` expression. mvdan's trace mentions `BuildExpr`/`newVertexRoot` but not closedness. |
| #4498 | inconsistencies in errors with empty structs during export (`-e '#test4'`) | closed (dup of #2565) | UNRELATED | Incomplete versus permanent errors. `field not allowed` is reported correctly there. |
| #4519, #4524, #4525, #4528, #4529 | `...`-opened fields staying open (explicitopen) | mixed | UNRELATED | They use `-e <field>` (a reference), which our bug does not affect. The mechanism is explicitopen and sharing. |
| #4501, #4502 | selecting a field from a value holding a definition stays open | open | UNRELATED | In-file selection semantics, not `BuildExpr`. |
| #3289 / #1327 | Value needs a method to finalize a value | closed / open | UNRELATED | A feature request. The `Syntax`+`BuildExpr` round trip is only mentioned as a workaround. |

## Queries (corpus `cs.py`)

```
'#4366|issues/4366'            -i 'a9f3f2a09|1238118|cross-context Finalize'
-i 'buildexpr|build_expr|BuildExpr' (all, then --since 2025-06-01)
-i --since 2025-09-01 '(^|\s|`)-e\s|--expression' '(not allowed|closed|closedness|typo check)'
-i --since 2026-05-01 'typo check' | 'GenerationMismatch|generation mismatch' | 'newVertexRoot'
   | 'adt\.Resolve' | 'TODO: Finalize here' | 'cross-context|different OpContext|new OpContext'
   | 'json\.Marshal\(this\)'
-i --since 2026-05-01 '(-e|--expression|BuildExpr)' '(no error|not reported|accepted|should fail|unexpected command success|succeeds)'
-i --since 2026-05-29 '(field not allowed|not allowed)' '(regress|v0\.16|no longer|used to|expected)'
-i --since 2026-05-29 '(BuildExpr|CompileString|Scope\()' '(Validate|closed|not allowed|definition)'
-i --since 2026-01-01 'cue def -e|def -e ' | 'eval -e .\{' | 'export -e .[#(]'
'#4349'
```

I also read the full threads of #4366, #4349, #2680, #3930, #4498, #4501, #4519, #4525, #4528 and #4529, and CL1238753. I checked the upstream log a9f3f2a09..master for cue/context.go, cue/types.go and the adt unify/eval files, and the bodies of all issues since 2026-05-29 that mention `-e`. I ran `-e 'close({}) & w'` on v0.16.1, v0.17.1, rc.3 and the fix. Files are in `tmp/prior-buildexpr-closedness/`.
