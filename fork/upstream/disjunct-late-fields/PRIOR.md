**Verdict: new issue (report only, no fix proposed).**

Checked on 2026-11-01 against the local corpus of cue-lang/cue's public record
(`tools/cs.py`): issues, PRs, comments, review comments, discussions, public
Gerrit, and the upstream log, which still equals v0.18.0-rc.3.

- Every upstream issue bisected to 339485ddf is closed, and its reproducer
  passes on rc.3. Shown by the pushdown-family prior-art checks (see
  `../let-pushdown/PRIOR.md` and `../pushdown-notify/PRIOR.md`).
- #4487 (updated by d47e4cee8) and #4520 are related, but their reproducers
  are unaffected. The disjunct-clone case (u5) is a separate bug with its own
  fix (`../disjunct-pushdown-notify/`).
- A search for disjunction plus comprehension plus lost or missing fields,
  since 2026-04, found nothing matching:

  `cs.py -i --since 2026-04-01 'disjunct|disjunction|\| *int' 'comprehension|if .*\{' 'missing|lost|lacks|drops|dropped|not present'`

Version results (repro.txtar): v0.16.1 passes; v0.17.1, rc.3 and the GraphOps
fork fail. The alias variant (`#A: #S | int`) gives a spurious cycle error on
rc.2 and rc.3 (from 082c52c57), and the silent loss again once
share-disjunction-safe's fix is applied.
