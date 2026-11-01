# Prior art: call-closedness-path

**Verdict: file, referencing #4270/#2782.** This is a new issue. No report covers user-function argument errors that lose their path within the argument after 7bf42f62f. The closest prior art is #2782 and #3186: builtins (json.Validate, json.Marshal) hide the inner path of a failing field, and "field not allowed" then names no field. Those are pre-existing, still open, and our fix does not touch them. Upstream's attempt at them, CL 1238666 "cue/errors: augment paths of wrapped errors", was reverted (a361a5b9d). Its follow-up, CL 1238752, was abandoned as "superseded by new CL". That attempt joined the inner path onto `Path()` (`y.Foo`), which is the option our README argues against, so maintainers may lean that way.

Checked 2026-11-01. The corpus covers up to 2026-10-30. A live `gh issue list` (updated ≥ 10-30) and the latest comments on Discussion #4484 (functions proposal) show nothing relevant.

## Candidates

| id | title | state | verdict | evidence |
|---|---|---|---|---|
| #2782 | vet does not expose enough error info when json.Validate fails | open (fix reverted 2026-06-10) | RELATED (same symptom class, builtin validator) | `y: error in call to encoding/json.Validate: field not allowed` names no field. It is identical on rc.1, rc.3 and our fix. It is not an argument error moved by `atCallSite`. |
| #3186 (dup #4129) | required field failure inside json.Marshal doesn't mention the field's name | open | RELATED (builtin drops inner path) | It is pre-existing and does not involve functions. mvdan links it to #2782. |
| #4270 | error deduplication removes distinct errors at same position | closed (774942dad, 2026-06-01) | RELATED (merging mechanism) | Since 774942dad, `removeMultiples` keeps errors whose messages differ. Ours merge only because losing the path makes position, path and message identical. The README's "(see #4270)" is accurate. |
| #4335 | make "field not allowed" errors more friendly | open | RELATED (orthogonal) | It asks that the message say what closed the struct. Its path already names the field (`staging.service1.volumes`). If the message named the field (mvdan's 2nd suggestion), `b` and `c` would be named but other below-argument errors would still lose their path. |
| 7bf42f62f tests (tighten.txtar `errorPaths`) | | master | not covered | They pin argument errors at the argument itself (`errorPaths.arg`), interpolation, and nested/result paths. No test has an error below an argument, so the loss was not deliberate. |
| #4538 | "matchN" call in definition does not work as expected | open | UNRELATED (matchN semantics, v0.17.1) | |

Our reproducer: rc.3 FAIL, `cue-up-call-error-argument-path` PASS. The candidate txtars and logs are in `prior-call-closedness-path/`.

## Queries (corpus `cs.py`)

```
-i --since 2026-08-01 --all '7bf42f6'
-i --since 2026-08-01 --all 'report errors arising in a call|calling field|call site'
-i --since 2026-08-01 --all 'atCallSite|moveToCallSite|callSiteOf|applyParamTypes|funcsig'
-i --since 2026-08-01 --all 'experiment\(functions\)'
-i --all 'augment paths of wrapped|#2782|issue/2782|Unroot'
-i --since 2026-09-01 --all 'func\(' 'not allowed'
-i --since 2026-09-01 --all 'func\(' 'error' '(path|position|reported)'
-i --all "(doesn.t|does not|without|no) (say|name|mention|tell)[a-z ]* (which|the) field"
-i --since 2026-06-01 --all 'removeMultiples|approximateEqual|dedup[a-z]* (error|distinct)|errors? (are|is) (merged|collapsed|deduplicated)'
-i --since 2026-09-01 --all '2782|wrapped error'
-i --since 2026-10-15 --all 'argument' 'error'
-i --since 2026-10-20 --all 'parameter' '(path|name)' 'error'
```

Also checked: the full threads of #4335, #4270, #2782, #3186 and #4538; CL 1238752 (abandoned) and its review; the 7bf42f62f golden diff; Discussion #4484 (latest comments via GraphQL); live `gh issue list`/`gh pr list`.
