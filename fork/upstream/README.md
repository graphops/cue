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

| Topic | Symptom on rc.3 | CUE lines | Since | Introduced by |
|---|---|---|---|---|
| [share-disjunction-safe](share-disjunction-safe/) | spurious "cyclic reference to field" | 7 | v0.18.0-rc.2 | `082c52c57` |
| [deferred-disjunct-scope](deferred-disjunct-scope/) | spurious "field not allowed"; invalid value exported | 6 | v0.18.0-rc.2 | `b68cd7163` |
| [call-closedness-path](call-closedness-path/) | "field not allowed" names no field; errors merged | 3 | v0.18.0-rc.2 | `7bf42f62f` |
| [let-pushdown](let-pushdown/) | conjunct silently dropped, wrong value exported | 11 | v0.17.0-alpha.2 | `339485ddf` |
| [try-finalize](try-finalize/) | try body constraint dropped; spurious cycle | 6 | v0.17.0-alpha.2 | `339485ddf` |
| [pushdown-notify](pushdown-notify/) | field silently lost on a referring struct | 10 | v0.17.0-alpha.2 | `339485ddf` |
| [comprehension-error-loss](comprehension-error-loss/) | export exits 0 with `[]` instead of an error | 16 | v0.17.0-alpha.2 (fatal case older) | `339485ddf` |
| [buildexpr-closedness](buildexpr-closedness/) | `cue export -e '#S & d'` accepts fields `#S` forbids | 3 | v0.17.0-alpha.2 | `a9f3f2a09` |

Every introducing commit was confirmed by building it and its parent. Every
root cause was confirmed with debug prints in a scratch build of rc.3.

Four of the eight trace back to `339485ddf` ("dependency-tracking
comprehension pushdown"): a field can complete, or be finalized, while a
pushed-down comprehension that targets it is still pending or running. The
fixes are local to each symptom; upstream may prefer to address the shared
cause.

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
