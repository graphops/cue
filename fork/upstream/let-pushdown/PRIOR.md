# Prior art: let-pushdown

**Verdict: file, referencing #4448/#4487/#4533.** This is a new issue. No report, open or closed, covers a let in a comprehension body keeping the comprehension from being pushed down. Every issue bisected to 339485ddf (cl/1236923) is closed, and each one's reproducer passes on rc.3 without our change. Two of those fixes are defeated by a let, though, so the same one-line change also covers variants of #4448 and #4487 that still fail on rc.3:

- **#4448** (fixed by d0182fcb6): upstream `issue4448.txtar` with `let v = 1` and `_w: v` in the body of `if spec != _|_ {}` fails on rc.3 with `invalid interpolation: non-concrete value _`. It passes with our fix. With `_w: 1` and no let, it passes on rc.3.
- **#4487** (fixed by d47e4cee8/a63d3d1a1): its reproducer with `let m = n` and `out: m` in the body of `if len(n) != 0 {...}` fails on rc.3 with the original `error in call to len: incomplete argument list (type list)`. It passes with our fix. A let in the `if false {...}` bodies of `q` does not trigger it.
- **#4533** (fixed by e147bfc48): its fix is what made our no-let variant pass. `own-nolet` fails on v0.17.1 and rc.2 and passes on rc.3. Its reproducers with a let added still pass on rc.3.

These are incomplete fixes rather than regressions: the let variants never worked after 339485ddf. One new issue that names all three fits better than a comment on each closed issue. The README now names #4487 and #4533. PR.md still names only #4448 in its body. Optionally add "and #4487" next to it. Its Fixes line does not change.

Checked 2026-11-01. The corpus runs to 2026-10-30, and upstream/master is a0768bd98, which matches `git ls-remote`. A live `gh issue list` (created ≥ 10-29, and comprehension issues updated ≥ 10-30) shows no new issues.

## Candidates

rc3 / let = `cue-v0.18.0-rc.3` / `cue-up-let-comprehension-pushdown`. The files are in `tmp/prior-let-pushdown/c/`.

| id | title | state | verdict | evidence (v0.17.1 / rc3 / let / notify) |
|---|---|---|---|---|
| own | repro.txtar | | | FAIL / FAIL / PASS / FAIL. Also with `CUE_DEBUG=sharing=0`: FAIL / FAIL / PASS / FAIL. |
| #4448 | v0.17: spurious "invalid interpolation: non-concrete value" | closed (d0182fcb6) | SAME-CAUSE (let variant) | Issue repro: FAIL / PASS / PASS / PASS. `v4448let`: FAIL / **FAIL / PASS** / FAIL. `v4448nolet`: FAIL / PASS / PASS / PASS. |
| #4487 | spurious incomplete len error in an if guard over a comprehension-built list | closed (d47e4cee8, a63d3d1a1) | SAME-CAUSE (let variant) | Issue repro: FAIL / PASS / PASS / PASS. `v4487let`: FAIL / **FAIL / PASS** / FAIL, with the same `len` error. `v4487let2` (let in `q`): passes on rc.3. |
| #4533 | spurious "cycle with field", or a dropped field, reading a let's field through an inline unification | closed (e147bfc48) | RELATED (its fix covers our no-let variant) | mvdan's two repros: PASS on rc3, let and notify. `v4533let`: PASS. `own-nolet`: FAIL on v0.17.1 and rc.2, PASS on rc3. |
| #4478 | let in a comprehension over the enclosing struct ("field set was already referenced") | closed (3d1cf46d6) | RELATED (let arc in frozen field set; different bug) | FAIL / PASS / PASS / PASS. Our PR cites 3d1cf46d6 for its reasoning. |
| #4520 | disjunction default not applied when read in a comprehension condition | closed (b218faaa6) | RELATED (bisected to 339485ddf; different mechanism) | FAIL / PASS / PASS / PASS. `v4520let`: FAIL / PASS / PASS / PASS. |
| #4423 / #4425 | field not allowed / "field set was already referenced" (guard reads a field a comprehension adds to) | closed (3aaa50470, f2197d46e) | RELATED (guard-driven premature finalization, same family) | #4425 repro and `v4425let`: all PASS. `v4423let`/`v4423nolet` (closedDef): all PASS. |
| #4422 | comprehension algorithm causes a hang | closed (1bf87f0f2) | UNRELATED (hang) | It terminates on all four. |
| #4376, #4440 | `let _x = self` misses comprehension fields; dynamic key via `let _self` | closed | UNRELATED (self-let binding) | FAIL / PASS / PASS / PASS. |
| #4392 | "field set was already referenced" for an if-inserted list inside a let | closed (906c3e961) | UNRELATED (let-bound struct, not a let in a body) | PASS on all. |
| #4474 | conflict with a let in a comprehension (stale `prev`) | closed | UNRELATED (let value reuse across iterations) | FAIL / PASS / PASS / PASS. |
| #4532 | let built by an empty comprehension is "incomplete" (rc.2) | closed | UNRELATED (b68cd7163) | PASS on all. |
| #4526 | default lost via reference to own definition (rc.2) | closed | UNRELATED (sharing) | PASS on all. |
| #4535 | top-level comprehension in a close() argument | closed (897d511cf) | UNRELATED | FAIL / PASS / PASS / PASS. |
| #4420 | "invalid interpolation" (let self) | closed (f6d78946a) | UNRELATED | PASS on all. |
| #3967 | evalv3 "undefined field" with cyclic conditionals | open | UNRELATED (pre-dates 339485ddf) | Fails on all, with the same error everywhere. |
| #2244, #3835, #3590/#3591 | older let-in-comprehension bugs | closed | UNRELATED (old evaluator / evalv3 era) | Read only. |

## Queries (corpus `cs.py`)

```
--all '339485dd'                 --all '1236923'
-i --all 'push.?down'
--all pushDownDeps | fieldConjunctsKnown | scheduleVertexConjuncts | addNotify | hasActiveParentTask
      | allAncestorsProcessed | insertConjunct | mayDeclareArcs | processRef
-i --all 'LetField'
--all 'v0.17.0-alpha.2'          -i --all 'alpha\.2' 'comprehension'
-i --since 2026-05-01 --all 'silently' 'comprehension|if '
-i --since 2026-05-01 --all 'dropped|drops|lost|missing' 'let' 'comprehension|\bif\b'
-i --since 2026-05-01 --all 'finali[sz]ed' 'default'
-i --since 2026-05-01 --all 'default' 'guard'
-i --since 2026-05-01 --all 'let' 'guard'
-i --since 2026-05-01 --all 'comprehension' 'let ' 'regression'
-i --all '\blet\b' 'comprehension' '(drop|lost|ignored|silent|missing)'
```

Also checked: a scan of the titles of all issues for "let" with comprehension/if/guard/for. Every non-PR issue since 2026-05-20 was listed and triaged by title. Open evaluator issues with comprehensions and code were scanned. I read the full threads of #4520, #4487, #4533, #4448, #4478, #4526, #4422, #4423, #4425, #4376, #4440, #4392, #4474, #4532, #4535 and #4420. I also read the upstream commit messages of e147bfc48, 154d2ce09 (else-clause pushdown, which made the same kind of change for else clauses but not for lets), b218faaa6, d0182fcb6, d47e4cee8, 7d035b83b, f2197d46e and 3d1cf46d6, and checked live `gh issue list`.

**At filing time** (when the issue number replaces `XXXX` in the commit): also
name #4487 next to #4448 in the commit body. With a let in the guarded body
(`let m = n`, `out: m` in `if len(n) != 0 {...}`), #4487's reproducer fails
again on rc.3 and passes with the fix, as #4448's does (`let v = 1`, `_w: v`).
