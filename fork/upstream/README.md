# Upstream drafts, batch 1

Issue drafts for eight fixes carried by the fork, prepared for proposing to
`cue-lang/cue`. Nothing here has been filed. Each directory holds:

- `repro.txtar`: a testscript asserting the correct behaviour. It fails on
  v0.18.0-rc.3 (identical to upstream master when written) and passes with
  only that fix applied (`fix/rc3-<topic>`) and on `graphops/v0.18.0-rc.3`.
- `README.md`: the issue text: reproducer, expected and actual output,
  versions, root cause, proposed fix, related upstream issues.

Run a reproducer with any `cue` on `PATH`:

```sh
go install github.com/rogpeppe/go-internal/cmd/testscript@v1.14.1
testscript fork/upstream/<topic>/repro.txtar
```

| Topic | Symptom on rc.3 | CUE lines | Since | Introduced by | Branch on cjorge-graphops/cue | Fork fix |
|---|---|---|---|---|---|---|
| [share-disjunction-safe](share-disjunction-safe/) | spurious "cyclic reference to field" | 7 | v0.18.0-rc.2 | `082c52c57` | `fix/share-disjunction-in-progress` | replaced |
| [deferred-disjunct-scope](deferred-disjunct-scope/) | spurious "field not allowed"; invalid value exported | 6 | v0.18.0-rc.2 | `b68cd7163` | `fix/deferred-completion-disjunct-scope` | replaced |
| [call-closedness-path](call-closedness-path/) | "field not allowed" names no field; errors merged | 3 | v0.18.0-rc.2 | `7bf42f62f` | `fix/call-error-argument-path` | replaced |
| [let-pushdown](let-pushdown/) | conjunct silently dropped, wrong value exported | 11 | v0.17.0-alpha.2 | `339485ddf` | `fix/let-comprehension-pushdown` | kept |
| [try-finalize](try-finalize/) | try body constraint dropped; spurious cycle | 6 | v0.17.0-alpha.2 | `339485ddf` | `fix/try-own-arcs-finalize` | replaced |
| [pushdown-notify](pushdown-notify/) | field silently lost on a referring struct | 5 | v0.17.0-alpha.2 | `339485ddf` | `fix/pushdown-notify-reference` | kept |
| [comprehension-error-loss](comprehension-error-loss/) | export exits 0 with `[]` instead of an error | 16 | v0.17.0-alpha.2 (fatal guard: v0.5.0-alpha.1, `748a68589`) | `339485ddf` | `fix/comprehension-error-fields` | replaced |
| [buildexpr-closedness](buildexpr-closedness/) | `cue export -e '#S & d'` accepts fields `#S` forbids | 3 | v0.17.0-alpha.2 | `a9f3f2a09` | `fix/buildexpr-finalize-context` | kept |

Every introducing commit was confirmed by building it and its parent. Every
root cause was confirmed with debug prints in a scratch build of rc.3.

Four of the eight trace back to `339485ddf` ("dependency-tracking
comprehension pushdown"): a field can complete, or be finalized, while a
pushed-down comprehension that targets it is still pending or running. The
fixes are local to each symptom; upstream may prefer to address the shared
cause.

## Status

Each topic is ready to file: `README.md` is the issue text, in upstream's bug
template, and `PR.md` the pull request text. Each branch holds one signed-off
commit on upstream master. The commit is written in upstream's style: earlier
changes are cited as `https://cuelang.org/cl/N`, and `Fixes #XXXX` stands for
the issue number to fill in. The test file names and test comments carry the
same placeholder. Upstream asks that an issue's approach be agreed before a
pull request is sent, so the order is: file the issue, wait for agreement,
fill in the issue number, then open the pull request.

Re-deriving each fix replaced five of the fork's fixes with the prepared
version ("replaced" in the table): the fork's were incomplete, too broad, or
larger than needed. The other three kept their code. See "Revised while
preparing upstream changes" in the fork's `fork/README.md`.

## Prior art

Each topic's `PRIOR.md` records a duplicate check against cue-lang/cue's whole
public record (as of 2026-11-01), searched with `tools/` and verified
behaviourally. The record covers:

- every issue, pull request and comment;
- every review comment and discussion;
- the public Gerrit, visible until about 2026-08;
- upstream master, which still equals rc.3.

Candidates' reproducers were run against rc.3 and each prepared fix.

- **None of the eight is known upstream: file all eight as new issues.**
  Each references the issues its introducing change fixed, plus the closest
  related ones.
- **let-pushdown:** a let in the guarded body defeats the upstream fixes for
  #4448 and #4487. This was verified, and the issue shows it.
- **try-finalize:** the `try` form of #4425 is another instance.
- **buildexpr-closedness:** it also undoes #2680 for `cue eval -e`.
- **share-disjunction-safe:** fixing it restores the older silent `x.args`
  loss, which 082c52c57 had turned into the cycle error. Its issue says
  so, and that loss should be filed alongside it.
- **call-closedness-path:** its issue defers on how inner paths are
  presented to the work on #2782. Upstream's last public change for that,
  https://cuelang.org/cl/1238752, was abandoned as superseded by a change
  that isn't public.
- **Side findings:** the builtin-argument error drop is a leftover of #4510,
  so it is a comment there, not a new issue. The other six are new
  (`_side-findings/PRIOR.md`).

Since about 2026-08, maintainers review their own changes privately. A fix
in progress would only show as a commit on master or a mention in an issue,
so re-run the check just before filing.

## Notes

- **Commit messages on the fork that these drafts correct.** The drafts
  describe what was verified:
  - `let-pushdown`: the early forcing goes through `OpContext.processRef`,
    not `Vertex.lookup`.
  - `try-finalize`: the body is dropped at `x`, already finalized, not at
    `x.a`.
  - `comprehension-error-loss`: the `else` case already fails on v0.17.1.
    The incomplete case passed on v0.16.1.
- Upstream issue #4486 (open) is the spurious cycle fixed by the fork's
  `try-target-probe`, which is not in this batch.
- `_side-findings/cases.txtar`: three problems found while reducing, none
  fixed by the fork, causes not analyzed:
  - `u5`: `out` loses a field that `x` has, although `out: x`. Correct on
    v0.17.1, wrong since v0.18.0-rc.1.
  - `p4`: spurious "field not allowed" against an open definition. Correct
    on v0.16.1, wrong since v0.17.1 at the latest.
  - `b5`: `cue export` panics with "not properly initialized" since v0.17.1
    at the latest, while `cue eval` prints values. v0.16.1 reports an
    error instead. The correct result may be a cycle error, but never a
    panic.
