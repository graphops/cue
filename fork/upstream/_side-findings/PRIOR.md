# Prior art: side findings

Checked 2026-11-01. The corpus runs to 2026-10-30, and upstream/master is a0768bd98 (= rc.3). Live `gh issue list` (updated ≥ 2026-10-30) and `gh search issues "not properly initialized"` found nothing newer.

Each case was run on v0.16.1, v0.17.1, rc.3, a build of upstream/master a0768bd98, master with `CUE_DEBUG=sharing=0`, and every prepared `cue-up-*` fix. Case files are in `tmp/prior-side/c/`. To find what to search for, each regression was bisected with `git bisect run`, between the merge base and the first bad tag. The parent of each introducing commit was rebuilt and passes. Bisect logs are in `tmp/prior-side/bis-*.log`. These bisections are not a root-cause analysis.

## Verdicts

| finding | known upstream? | verdict | introduced by | notes |
|---|---|---|---|---|
| u5: `out` loses a field of `x` | **no** | new issue | d47e4cee8 "settle a pending arc once its comprehension is done" (Updates #4487; private CL), between v0.17.1 and rc.1 | Wrong on master, also with `sharing=0`. No prepared fix changes it. |
| p4: spurious `field not allowed` against `#M: {...}` | **no** | new issue | a10029dad "skip Disjunctions on ancestor-cycle arcs" (cl/1237537, no issue), in v0.17.0-alpha.2 | Fails on master, also with `sharing=0`. The follow-up 587dbbc9a (no issue) does not fix it. |
| b5: export panic `not properly initialized` | **no** | new issue | 339485ddf "dependency-tracking comprehension pushdown" (cl/1236923), in v0.17.0-alpha.2 | Panics on master, also with `sharing=0`. |
| json.Marshal guard drops `<=2` | **no** | new issue | 339485ddf | Only when the builtin takes the whole struct. `if x.a == 5`, `if len(x) > 0`, `if json.Marshal(x.a) != ""` and `if (x & {}) != _\|_` all still report the bound on master. |
| `x.y` field lost through a selected reference | **no** | new issue | not a regression (v0.16.1 has it too) | `a` lacks `v` on master, also with `sharing=0` and with `cue-up-pushdown-notify-reference`. The same happens with `x: y: v: 1` written directly in the guard (`xy3.cue`). |
| builtin-argument error dropped | **yes, as a residue of #4510** (closed) | comment on #4510 | n/a | See below. |
| `out: x.args & #A` loses `b` | **no** | new issue | 339485ddf | On rc.2+ hidden behind a spurious cycle error. See below. |

## Details and candidates

**u5.** The introducing commit is the fix for #4487's family: it updates #4487, with a63d3d1a1 and 621353368 in the same series. #4487 has no comments. No issue mentions d47e4cee8. The closest report is #4533 ("a dropped field, reading a let's field through an inline unification"). It is RELATED (another silent field drop since v0.18), but it bisects to d0182fcb6, and its fix e147bfc48 is on master while u5 still fails. The share-disjunction-safe commit (082c52c57, #4530) does not cause it.

**p4.** No issue, comment or discussion mentions a10029dad or cl/1237537. The only mentions are cl/1237537's own Gerrit thread and the commit message of 587dbbc9a ("apply the kinds of disjunctions skipped on a cycle"). #4022 (closedness regression with disjunctions and ellipses, an aliased `null | #Def`) is RELATED but closed, and it passes on every build. #4423 and #4508 are UNRELATED: #4423 is guard-driven finalization, and #4508 is explicitopen working as intended.

**b5.** The panic message appears only in old, closed reports: #3648 (list.Concat, 2025), #4134 (matchN via Expr) and #500. None of them has this shape. Other panics from the 339485ddf series (#4521 "vertex not initialized", #4419) have different messages and shapes, and both are fixed.

**json.Marshal guard.** RELATED: #4423 (closed; 3aaa50470 and f2197d46e), the "guard-driven premature finalization" class from 339485ddf. Its `selfInsert` case, a guard on `local.zone` that adds to `local`, passes, and the reads that take a field or `len` also pass. Also RELATED: #4366 and a9f3f2a09, which are about json.Marshal finalizing an in-flight vertex in a fresh OpContext, but they concern the typo check, not dropped conjuncts. No report has a builtin that marshals the struct its guard adds to.

**`x.y` field loss.** The pushdown-notify topic has the same symptom (a referring field misses what a comprehension adds after its guard resolved the reference). This case is older and not fixed by that change, so it is a separate bug. #2555 (open, file order) and #3296 (open, default choice with a guard) are UNRELATED.

**Builtin-argument error drop.** #4510 ("a used scalar of an inline struct or let is not checked against its `!=`, `=~` or builtin validators") is closed by b68cd7163, with 56d5c165e and de8f999a9. Its upstream test `issue4510: consumers: errBuiltin: strings.ToUpper((#C & {id: "storage"}).out)` covers the inline call, and rc.3 rejects it (v0.17.1 did not). What remains, on rc.3 and master:

```cue
let L = {o: strings.ToUpper((#C & {id: "storage"}).out)}   // also len(...)
r: L.o
```

This exports `"K/STORAGE"`, and `len(...)` exports 9. It errors as expected for a regular field (`x: {o: ...}; r: x.o`), for `r: L` (the whole let), for a let of the bare call, for a non-builtin consumer (`... .out + "y"`), and with `CUE_DEBUG=sharing=0`. So structure sharing plays a part. A nested let (`let C = #C & {...}`, `let L = {o: strings.ToUpper(C.out)}`) loses it too. The only related todo in `nonrooted.txtar` is the comprehension let clause read only by an expression (`compLet.expr`), which is a different shape. No issue or comment mentions this case. The "inside a disjunct within an inline struct" form from the README could not be reconstructed (`({o: …} | int).o` is an invalid-operand error), so it still needs its probe file.

**`x.args` loss.** v0.16.1 is correct, v0.17.1 silently drops `b` (339485ddf), and rc.3 and master report `x.args: cyclic reference to field b` instead. That error comes from 082c52c57: `cue-up-share-disjunction-in-progress` and master with `sharing=0` both return to the silent v0.17.1 result. So the share-disjunction-safe fix uncovers this silent loss again. u5 is RELATED (same shape, but bisects to d47e4cee8). No issue matches.

## Queries (corpus `cs.py`)

```
-i --all 'not properly initialized'      -i --since 2025-10-01 'state: unprocessed|unprocessed' | 'appendJSON' | 'panic'  +'export'
-i --since 2026-05-01 '(missing|drops?|dropped|loses|lost|silently)...(field|arc)|(field|fields)...(missing|dropped|lost|disappear)'
-i --since 2026-06-01 '(structure sharing|shared|sharing)' '(disjunction|disjunct)' '(comprehension|for |if )' '(missing|dropped|lost|loses|drops)'
--since 2025-06-01 '\{\.\.\.\}\s*\|\s*(int|string)|#\w+\s*\|\s*(int|string)\b' '(out|x):\s*x'
-i --since 2025-06-01 'not allowed' '\{\s*\.\.\.\s*\}' '\|'
-i --since 2025-06-01 '(open definition|definition is open|ellipsis|\.\.\.)' 'field not allowed' '(disjunct|disjunction)'
-i --since 2026-05-20 'field not allowed' '\.\.\.' 'alias|#\w+:\s*#\w+\s*\|'
-i --since 2026-05-20 'for \w+, \w+ in (out|\w+)\s*\{' '(out|\w+):\s*x\b'
-i --all 'a10029dad|1237537'   --all '587dbbc9a|skipped on a cycle|ancestor-cycle|ancestor cycle|hasAncestorCycle'
--all 'd47e4cee8|settle a pending arc|#4487|issue/4487'
-i --all '339485ddf|1236923|comprehension pushdown|pushed-down comprehension|pushdown'
-i --since 2025-06-01 'if\s+(json|yaml)\.Marshal'      -i --all 'json\.Marshal\(\w+\)\s*!='
-i --since 2025-06-01 '(json|yaml)\.Marshal' '(ignored|not applied|not enforced|no error|silently|lost|dropped|out of bound)'
-i --since 2025-06-01 '(insertConjunct|finalized vertex|already finalized|after finaliz|into a finalized)'
-i --since 2025-09-01 'if\s+(len|json\.Marshal|...)\((\w+)\)' '(silently|ignored|no error|...)'
-i --since 2026-01-01 '(constraint|bound|validator)...(lost|dropped|ignored|not applied|not enforced)'
--all '\w+:\s*\w+\.\w+\s*&\s*\{' 'if \w+\.\w+\s*\{' '(missing|lack|lost|...)'
-i --all '(selector|selection|selected|x\.y)' '(comprehension|if )' '(field|arc)...(missing|lost|not (included|propagated|visible))'
-i --all 'strings\.ToUpper\(\('    -i --since 2026-05-20 'strings\.\w+\(\(#'    --all '#4510|issues/4510|b68cd71'
-i --since 2026-05-20 'let \w+ = \{' 'builtin' 'error'
-i --since 2026-09-01 'rc\.1' 'regression'
```

I also read the full threads of #4533, #4530, #4510, #4487, #4423, #4022, #4508, #4478, #4521, #4419, #3648, #2718 and #2555 (head). I read the commit messages of d47e4cee8, a10029dad, 587dbbc9a, 3aaa50470, 1bf87f0f2, b218faaa6, 154d2ce09, baaa0e695, fd3117d83, 4232b8d36, 1c287ec21, 5893a5615 and 19d6c62c0. I triaged by title every non-PR issue since 2026-09-01, plus the public Gerrit CLs still pending, and checked the upstream tests `nonrooted.txtar`, `issue4423.txtar` and `disjunctions/edge.txtar`. The sibling PRIOR.md files for pushdown-notify and share-disjunction-safe list the other 339485ddf and 082c52c57 issues (#4520, #4526, #4532, #4422, #4425), all closed and passing on rc.3.
