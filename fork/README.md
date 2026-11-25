# GraphOps fork of CUE v0.18.0

Evaluator fixes and additions GraphOps needs to run its configurations on CUE
v0.18.0, until equivalent changes land upstream. See `GRAPHOPS.md` on `main`
for the fork's branch conventions. No release has been tagged from this branch
yet; [Release notes](#release-notes) has the draft for the first one.

## Layering

The branch starts at the `v0.18.0` tag and has four layers, in this order:

1. **Foundation.** The content of the dusk-to-dawn integration branch
   `integration/v0.18.0` (722349892, tree 8650133d5): seven upstream `master`
   commits that v0.18.0 lacks, 21 upstream-bound fixes, three upstream-bound
   performance changes, and one commit that updates the recorded results of
   their combination. Each is one commit here, ported from the integration
   branch with its message; the fixes and performance changes carry the
   fork's identity and trailers, and the upstream commits keep their author
   and message and name the upstream commit and what their port to v0.18.0
   leaves out. The tree after the foundation is the integration branch's tree.
2. **GraphOps patches.** Fixes and additions that are not meant for upstream
   as they are, or that upstream fixed only after v0.18.0, from the branches
   listed in [`patches.txt`](patches.txt).
3. **Combined goldens.** One commit, `cue/testdata: update recorded results
   for the fork patches combined`, for the recorded results that change only
   when the patches meet the foundation; see
   [Effects of combining](#effects-of-combining).
4. **Fork files.** The release workflow, CI and goreleaser configuration as on
   `graphops/v0.18.0-rc.4`, and this directory.

[`rebuild.sh`](rebuild.sh) builds the branch again from the integration branch
and the patch list; see [Rebuilding](#rebuilding).

## Foundation

| Commit | What it fixes or changes |
|---|---|
| upstream `6b239f555` | `walkEscapes` counts the extra level of `self`. Needed by call-memo |
| upstream `cd43cfbec`, `8c4aa86de` | A validator's errors no longer leak into the next validator (the validator half of the former validator-error-leak) |
| upstream `7d575eaaf`, `f6483025c` | Wrong result, the fix for #4598: closures of a pattern that differ only by label shared one call result (`p: [string]~(K,_): func() -> string: K`, `out: [for k, _ in p {p[k]()}]` gave `["a","a"]`, now `["a","b"]`). Replaces the fork's call-memo-label; perf/call-memo keys the callee by its `funcEnvKey` |
| upstream `8fbe49d7d`, `582825123` | Incomplete value, order dependent, the fix for #4595: iterating a struct before it is evaluated validated an arc a comprehension declares while its reference was unresolved, fixing it to `_`. Replaces the fork's arctype-validate |
| `fix/v0.18.0-walk-try-escapes` | `walkEscapes` gave up on a try clause, so equal closures over a try clause were kept apart (`mkTry(1) & mkTry(1)` conflicted) |
| `fix/v0.18.0-call-arg-cycle` | Spurious structural cycle: a call inherited the cyclic mark of its conjunct and was postponed although its arguments do not reach the cycle. Since `a1cfb1ca4` (v0.13.0-alpha.2). Stacked on walk-try-escapes |
| `fix/v0.18.0-deferred-completion-disjunct-scope` | Spurious "field not allowed" and invalid values accepted: a deferred completion inside a disjunct was recorded on a task outside it. Since `b68cd7163` (v0.18.0-rc.2) |
| `fix/v0.18.0-disjunct-cycle-skip` | Silently dropped disjunction: a field looked up from within a value it unifies (`x: {a: 1, b: out.a}`, `out: x & #A`) had its disjunction skipped, as if the lookup were a cycle to a parent, so `#A` applied only its kinds. Since `a10029dad` (https://cuelang.org/cl/1237537, v0.17.0) |
| `fix/v0.18.0-disjunct-pushdown-notify` | Silent value loss: the arcs cloned for a disjunct lost their parent tasks, so fields a comprehension declares were missing in the disjunct. Since `d47e4cee8` (v0.18.0-rc.1) |
| `fix/v0.18.0-comprehension-dependency` | A pushed-down comprehension holds the fields it declares until it is done: a field no longer completes before the comprehension has run, and fails when its clauses fail (`x: [...]` next to `if b == 1 {x: [1]}` exported `[]`). Since `339485ddf`. Stacked on disjunct-pushdown-notify |
| `fix/v0.18.0-export-let` | `cue eval` and `Value.Syntax` printed a copy of the value for a `let X = self` referenced outside the struct declaring it |
| `fix/v0.18.0-fmt-alias-form` | `cue fmt` wrote `a~(X)` for files whose language version predates it |
| `fix/v0.18.0-let-comprehension-pushdown` | Silent value loss: a `let` in a comprehension body kept it from being pushed down. Since `339485ddf` |
| `fix/v0.18.0-lookup-error-leak` | Wrong results: a scalar comparison took the incomplete error a required-field reference had recorded for its node, so equal scalars conflicted (`a!: "x"`, `b: a`, `b: "x"`). The other half of the former validator-error-leak and context-error-leak fixes, which upstream `8c4aa86de` now covers |
| `fix/v0.18.0-nested-disjunction-errors` | Lost diagnostics: when every disjunct failed only through a nested disjunction, the error was a bare "empty disjunction" |
| `fix/v0.18.0-pushdown-notify-reference` | Silent value loss: a reference that copied a field while a comprehension pushed down to it was still pending or running missed the conjuncts it added later (#4550) |
| `fix/v0.18.0-running-producer` | Silent constraint loss: the arc type a running comprehension may still decide was frozen, so a try body reading it was discarded |
| `fix/v0.18.0-try-speculative` | Silent constraint loss and spurious cycles: while a try clause pre-evaluates its body, nothing it has yet to yield into is finalized, has its disjunction resolved, or has its field set or arc type frozen (#4486). Stacked on running-producer |
| `fix/v0.18.0-closed-list-disjunct` | Spurious "field not allowed" for list elements: a comprehension in a list literal that refers to the field the list is unified into, under a disjunction, made the field keep the closedness of the definition but not the disjunct that opened it (`#S: [...]`, `#A: #S \| int`, `x: [1, if out[0] == 1 {2}]`, `out: x & #A`). Since evalv3 (v0.13.2) |
| `fix/v0.18.0-share-disjunction-in-progress` | Spurious "cyclic reference to field" or lost fields: a field sharing a vertex finalized it when it met a disjunction, also from a partial evaluation. Since `082c52c57` (v0.18.0-rc.2). Stacked on closed-list-disjunct |
| `fix/v0.18.0-share-funcarg-chain` | Spurious structural cycle: a function argument that structure shares another vertex was scheduled under the callee's cycle-reference chain. The order of fields stays as without the fix; see [Field order of a function argument](#field-order-of-a-function-argument) |
| `fix/v0.18.0-share-late-conjunct` | Stale value: a field that shares a vertex kept leading references to it after a comprehension added a conjunct (#4524). Since `339485ddf` |
| `fix/v0.18.0-struct-comprehension-top` | A comprehension that yields nothing embeds nothing: `_ & {if false {1}}` and a parameter `v: _` given an empty comprehension are `{}` instead of `_` (#4577) |
| `fix/v0.18.0-test-eq-error` | Test harness: `@test(eq)` accepted an erroneous struct that still has arcs, such as an empty disjunction, so wrong results could pass as expected |
| `fix/v0.18.0-toposort-stable-sort` | Field order depending on where the module cache is |
| `perf/v0.18.0-call-constraint-replay` | Performance: the parameter constraints of a call are added once per activation, not again for every use of its result |
| `perf/v0.18.0-call-memo` | Performance: memoized calls are keyed by what their arguments read, so copies of a struct holding a call, and comprehension iterations, share its result. Replaces the fork's own call memo (call-memo-copies, call-memo-bindings). Stacked on call-constraint-replay |
| `perf/v0.18.0-share-past-def` | Performance: structure sharing is kept past a definition the shared value already applied; a vertex of a function argument keeps sharing only as before, so that its fields keep their order (see [Field order and sharing past a definition](#field-order-and-sharing-past-a-definition)). Replaces the fork's `feat/v0.18.0-share-past-def`. Stacked on share-disjunction-in-progress (with closed-list-disjunct under it), and cherry-picks of share-funcarg-chain and share-late-conjunct |
| `integration: update recorded results for combined fixes` | Recorded results that change only when the above are combined |

Each fix and performance change also exists on its own branch, named in the
first column: one commit on `fix/v0.18.0-upstream-prereqs` (v0.18.0 with the
seven upstream commits, which are the line's own commits), or on the branch
the row says it is stacked on, with the recorded results of that base. Each
passes `CUE_UPDATE=1 go test ./...` and then `go test ./...` on its base.
Their code changes are those of the integration branch's commits, line for
line; only recorded results differ where the base lacks other fixes. The
previous heads are kept under `refs/candidate2/<branch>`, and those of
`fix/v0.18.0-share-funcarg-chain` and `perf/v0.18.0-share-past-def` before the
field order fix under `refs/candidate3/<branch>`, and that of
`perf/v0.18.0-share-past-def` before its own field order fix under
`refs/candidate4/<branch>`;
`fix/v0.18.0-arctype-validate`, `fix/v0.18.0-call-memo-label`,
`fix/v0.18.0-context-error-leak` and `feat/v0.18.0-share-past-def` are
retired there, as upstream commits or the rows above replace them.

## GraphOps patches

| Branch | What it does | Why it is still carried |
|---|---|---|
| `feat/v0.18.0-marshal-sorted` | `json.MarshalSorted`, `yaml.MarshalSorted`, `Value.MarshalJSONSorted`: `json.Marshal` and `yaml.Marshal` with the fields of every struct ordered by key at every depth, so that content hashed from the result does not change when a declaration moves. Tests: `pkg/encoding/json/testdata/marshalsorted.txtar`, `pkg/encoding/yaml/testdata/marshalsorted.txtar`, `cmd/cue/cmd/testdata/script/marshal_sorted_incomplete.txtar` | Fork-only addition; upstream master (`e1be5ff5a`) has no equivalent. The blueprints call it |
| `fix/v0.18.0-struct-data-incomplete` | `struct.Data(v)`: `v` as concrete data (regular fields, defaults resolved, no closedness, definitions, hidden or optional fields), built as fresh CUE literals, an in-CUE replacement for `json.Unmarshal(json.Marshal(v))` that keeps CUE kinds such as bytes; and its fix: an argument that is only incomplete keeps its error code. Tests: `pkg/struct/testdata/data.txtar`, `cmd/cue/cmd/testdata/script/struct_data_incomplete.txtar` | Fork-only addition; not on upstream master |
| `feat/v0.18.0-build-shallow` | `Context.BuildInstanceShallow`, `Value.LookupPathShallow`, `Value.ShallowSelectors`: Go API for reading a few fields of a large package without finalizing all of it. Test: `cue/shallow_test.go` | Fork-only addition; not on upstream master |
| `fix/v0.18.0-buildexpr-closedness` | Upstream `0beee1028` and `42875a302` (#4573), cherry-picked with `-x`: a value built with `Context.BuildExpr`, as by `cue eval -e '#S & d'`, skipped definition closedness; plus the fork's earlier tests for nested and `cue export -e` cases | Fixed on upstream `master` after v0.18.0; no v0.18.x release or release branch has it. Drop once the base includes `42875a302` |
| `fix/v0.18.0-call-closedness-path` | Lost diagnostics: an error below a function argument was reported at the call without its path within the argument (`w: cfg.extra: field not allowed`). Since `7bf42f62f` (v0.18.0-rc.2). Tests: `functions/argpath.txtar`, `argumentPaths` in `functions/tighten.txtar` | Not fixed upstream (master reports `extra: field not allowed`); upstream is reworking error paths (#2782) |
| `fix/v0.18.0-late-conjunct` | Silently accepted invalid configurations: a conjunct added to an arc after it was finalized was dropped without a trace; it is now checked against the arc and reported if it conflicts with it or narrows it. The third commit (`af81bc541`) also checks a conjunct that reaches an arc after its disjunctions were processed, which the arc takes but which does not reach its value. Test: `comprehensions/lateconjunct.txtar` | An interim detector; the cause is upstream scheduler design (#4579, no fix upstream). The foundation fixes none of the cases it reports |
| `fix/v0.18.0-try-closed-field` | Spurious "undefined field", order dependent: `d.y?` on a closed struct that does not declare `y` failed instead of discarding the try. Test: `comprehensions/tryclosed.txtar` | A semantics question for upstream; master still fails `tryLast` and `viaDefault` |
| `fix/v0.18.0-embed-root` | `@embed` followed symbolic links out of the module root; every path is now resolved with `os.Root` on the module root. Tests: `cue/load/embed_test.go`, `cmd/cue/cmd/testdata/script/embed_symlink.txtar` | Upstream considers following links intended (#3299); master still follows them |

Not carried from `graphops/v0.18.0-rc.4`: the fork's call memo
(`fix/rc4-call-memo-copies`, `fix/rc4-call-memo-bindings`) and the sharing
addition (`feat/v0.18.0-share-past-def`), which `perf/v0.18.0-call-memo`,
`perf/v0.18.0-call-constraint-replay` and `perf/v0.18.0-share-past-def`
replace; the fork's arctype-validate, call-memo-label and context-error-leak
fixes, which upstream `582825123`, `f6483025c` and `8c4aa86de` (with
lookup-error-leak) replace; and the rc.4-era try-finalize,
try-share-disjunction, try-target-probe, ready-parent-completion and
comprehension-error-loss fixes, which the foundation's try-speculative,
running-producer and comprehension-dependency replace.

### Still applicable (2026-11-26)

Each patch was checked against upstream `master` at `e1be5ff5a` (there is no
v0.18.x release after v0.18.0, and no `release-branch.v0.18` beyond the tag)
and against the foundation. A testscript per patch, run with four builds
(the foundation alone, this branch, upstream master, `v0.18.0-rc.4-graphops.2`;
pass = the patch's behaviour):

| Patch | Foundation | This branch | master | graphops.2 |
|---|---|---|---|---|
| marshal-sorted (`marshal_sorted_incomplete.txtar`) | FAIL | pass | FAIL | pass |
| struct-data-incomplete (`struct_data_incomplete.txtar`) | FAIL | pass | FAIL | pass |
| buildexpr-closedness (`eval_e_closedness.txtar`) | FAIL | pass | pass | pass |
| call-closedness-path (`cfg.extra`, `cfg.opts.porrt`) | FAIL | pass | FAIL | pass |
| late-conjunct (#4579's `b.cue`: `x.out: 2` from a comprehension after `x` was finalized) | FAIL | pass | FAIL | pass |
| try-closed-field (`tryFirst`, `tryLast`, `viaDefault`) | FAIL | pass | FAIL | pass |
| embed-root (`embed_symlink.txtar`) | FAIL | pass | FAIL | pass |

The patches' own Go tests on the foundation with their code left out:
`TestEvalV3/functions/argpath` fails 8 of its 9 cases,
`TestEvalV3/comprehensions/lateconjunct` 28 of 63 (among them `field`,
`newField`, `narrowStructDefault`, `lateDisjunctField` and the
`validator*` cases), `TestEvalV3/comprehensions/tryclosed` 3 of 4, and
`TestFromExpr`, `TestBuildExprClosedness` and `TestScript/eval_e*` fail.
This branch without the code of `af81bc541` fails `narrowStructDefault` and
`lateDisjunctField`, so the third late-conjunct commit is still needed.

The new foundation fixes do not interact with the patches: every patch
commit and the combined-goldens commit apply as they did on the previous
foundation (`git range-diff` shows them unchanged), and `go test ./...`
passes with the recorded results unchanged, including `lateconjunct.txtar`,
`tryclosed.txtar` and `argpath.txtar` under test-eq-error's stricter
`@test(eq)`, and the tests of disjunct-cycle-skip
(`cycle/disjunctionlookup.txtar`) and closed-list-disjunct
(`comprehensions/listdisjunct.txtar`) with the late-conjunct check active.
No patch is dropped.

## Effects of combining

The combined-goldens commit records these, each with its reason in the commit
message:

- `functions/memoprobes.txtar` (a foundation test): the errors that arise in a
  function argument name their path within it (`n.b: field not allowed`), as
  call-closedness-path does for every call. A build without any memo lookup
  gives the same errors, and the memo stores only results computed without an
  error.
- `comprehensions/lateconjunct.txtar`: with comprehension-dependency, the
  second comprehension holds `n.a` until it is done, so a guard that does not
  need `n.a.size` no longer finalizes it: `narrowDefault` and `agreeDefault`
  take the yielded value, the correct result, instead of reporting it as late
  or dropping it. `narrowStructDefault`, whose guard resolves the struct
  disjunction of `n.a` while it is held, lost the yielded `{x: 2}` without an
  error. The third late-conjunct commit reports it: a conjunct that reaches a
  node after its disjunctions were processed does not reach the disjuncts, and
  is checked as a dropped one. It is part of `fix/v0.18.0-late-conjunct`,
  where the guard finalizes `n.a` instead, so its tests pass there with or
  without it; no other test result changes with it on either base.
- `comprehensions/pushdown_let.txtar`, `letEmbed`: the conjunct a comprehension
  with an embedded let drops is reported by the late-conjunct check instead of
  being lost. The correct result stays recorded as `eq:todo`.
- `comprehensions/trypreevaluate.txtar`, `guardAfterTry` and `guardWithoutTry`
  (invalid programs): the error of `x` is reported by the late-conjunct check,
  "adding field b not allowed as field set was already referenced", instead of
  as an incomplete cyclic reference.

Checked for and not found: try-closed-field with try-speculative (a try on a
field a closed struct lacks, in its body, its else clause, nested, through a
default disjunct, and with the field added by the else clause, give the
results of each fix alone), and call-closedness-path with call-memo (the
errors of `argpath`, `tighten`, `memoprobes`, `memocopies` and `memobindings`
are the same with and without memo lookups).

## Field order of a call result taken by an opened copy

Against `v0.18.0-rc.4-graphops.2`, one blueprint render differs only in the
order of keys inside a string: a JSON document kept in a Secret, made with
`json.Marshal`, lists one label in another position. The decoded values are equal, but the Secret's bytes change.

The cause is graphops.2's own call memo, not a change of this branch. Its
first commit, `b19833ce1` ("share call results between copies of a struct",
`2ba9de095` on v0.18.0), lets the opened copy reuse the result of the equal
call made for `out.resources`, with that evaluation's field order. Every build
without it gives the order this branch gives: v0.18.0-rc.4, v0.18.0, upstream
master, this branch, and this branch without `perf/v0.18.0-call-memo`. Found
by elimination, rendering `coordinated-rollout` once per build: this branch
with any of these left out still renders this branch's bytes:
toposort-stable-sort; call-constraint-replay; share-past-def;
struct-comprehension-top; upstream `582825123` and `8c4aa86de`; the twelve
fixes from deferred-completion-disjunct-scope to try-speculative;
closed-list-disjunct, share-disjunction-in-progress, share-funcarg-chain and
share-late-conjunct together with share-past-def; every GraphOps patch but
the two `pkg` additions the blueprint calls; `graphops/v0.18.0-candidate1` with the fork's memo
renders graphops.2's bytes, also with `6b239f555` and call-constraint-replay
added, and with the fork's memo replaced by `f6483025c`, call-constraint-replay
and `perf/v0.18.0-call-memo` it renders this branch's bytes.

Both orders are valid: `component` and the platform labels come from
different structs, with no declared order between them, and CUE does not
define one. This branch's order is the evaluator's own, so it is a correction
of a memo artifact rather than a regression; no upstream report is needed.
The effect for deployments is one change of the Secret's bytes when moving
from graphops.2. [`keyorder.txtar`](keyorder.txtar) is a self-contained
reproducer (`testscript`, with the binary under test as `cue`): it passes with
v0.18.0-rc.4, v0.18.0, master and this branch, with or without its call memo,
and fails with graphops.2.

## Field order of a function argument

The first version of `fix/v0.18.0-share-funcarg-chain` changed the field order
of values that a function builds from an argument: the fields the function
body adds came before the argument's own, against the order the argument
declares them in (`metadata, kind` instead of `kind, metadata`;
`labels, annotations, name, namespace` instead of `name, namespace, labels,
annotations`). The values were equal. In the sweep of
`graphops/v0.18.0-candidate3` this accounted for 77 of the 93 scenarios that
differed from graphops.2.

The cause: the `toposort` package orders the structs of a vertex by the first
reference through which each was reached, the last entry of the conjunct's
cycle-reference chain. When a callee reached an argument that structure
shares another vertex through a selection or a comprehension, the fix
replaced the chain with that of the argument, whose first reference is the
one inside the call's argument list (`_phases` in `resources({ctx, phases:
_phases})`), after the call itself. The argument's struct then sorted after
the body's, instead of tying with it at the call. The fixed version (dusk-to-dawn
`1436f8daa`) keeps both: the reference starts a chain that continues with the
argument's for cycle detection and keeps the chain it was resolved under,
which `toposort` follows (`RefNode.Outermost`). The fix's own tests pass
unchanged, with the same recorded statistics, and
`cmd/cue/cmd/testdata/script/export_funcarg_order.txtar` checks the order.

[`fieldorder-funcarg.txtar`](fieldorder-funcarg.txtar) is the self-contained
reproducer: v0.18.0, upstream master, graphops.2 and this branch pass;
`graphops/v0.18.0-candidate3` fails. Upstream's own order is not consistent
here: a direct call `apply(m, _phases.policy)` puts the body's fields first on
master too, as does master with `CUE_DEBUG=sharing=0` for the call through a
comprehension. The fix keeps master's order rather than choosing between
them.

## Field order and sharing past a definition

`perf/v0.18.0-share-past-def` keeps a node sharing when a definition arrives
that the shared value already applied. Its first version in this line did so
for vertices of function arguments too, and changed the order of fields in 17
blueprint scenarios against graphops.2: a parameter `m: #M`, given a value
that applied `#M` and embedded in the body before a field the body adds, put
the argument's fields first. A vertex of an argument holds conjuncts that
carry the chain of the argument, and `toposort` orders its fields by that
chain when the vertex evaluates them; through sharing a reader ordered them
by its own chain. The change now leaves such vertices to the narrower rule
the fix for upstream #4512 applies already (a definition the shared vertex
names as a direct conjunct). `cmd/cue/cmd/testdata/script/export_sharepastdef_order.txtar`
checks the order. The gains of the schema chains stay: 300 levels of
`child: B & {ctx: parent.ctx}` take 909 unifications and 2714 conjuncts
(1208 and 93311 without the change), and the definition-first chain 312 and
614 (1807 and 92407). The gain for a parameter declared with a definition is
given up: 30 nested calls `F(F(...))` with `F: func(v: #D) -> #D: v` take 236
and 40868, as without the change (179 and 2097 before).

Sharing past a definition still changes the field order of most stack
outputs against the same build without it (146 of 900 scenarios): a node
that keeps sharing exports the order of the vertex it shares, not the order
its own conjuncts would give. graphops.2's own share-past-def gives the same
order in all but three of them (a group of three in [Validation](#validation)),
so against graphops.2 this is no change; leaving the change out would change
those 146 instead, and double the time and memory of ten stack scenarios
(137 s and 2969 MiB peak against 67 s and 1455 MiB over their ten scenarios).
Upstream sharing has the same property: on master, a function result whose
argument the callee reaches through a selection or a comprehension orders its
fields differently with `CUE_DEBUG=sharing=0` (a report is prepared in the
dusk-to-dawn drafts). Making a shared node order its fields as its own
conjuncts would is a question for upstream's `toposort` design.

## Validation

On 2026-11-26, at the commit that adds this file:

- `go vet ./...` and `go test ./...` (121 packages) pass. The GraphOps tests
  are part of them.
- `p: [string]~(K,_): func() -> string: K`, `p: {a: _, b: _}`,
  `out: [for k, _ in p {p[k]()}]` exports `["a","b"]`
  (`v0.18.0-rc.4-graphops.2` gives `["a","a"]`).
- Every fix and performance branch passes `CUE_UPDATE=1 go test ./...` and
  `go test ./...` on its own base, and so does every GraphOps patch branch on
  v0.18.0.

- The full blueprint sweep (approved by the owner): every scenario of every
  blueprint of GraphOps's blueprint repository (`main`, fetched on
  2026-11-26),
  900 scenarios (163 blueprints, each without an overlay and with each
  `testdata/*.cue`), rendered with this branch (`cd15d7864`, before this
  file's last change), with `v0.18.0-rc.4-graphops.2`, and with this branch
  without `perf/v0.18.0-share-past-def`, by [`sweep.sh`](sweep.sh), one
  render at a time, capped at 4 GB. No render was killed.

  | | This branch | graphops.2 | Without share-past-def |
  |---|---|---|---|
  | Renders that succeed | 555 | 555 | 555 |
  | Renders that fail (the same 345 scenarios, mostly `invalid-*` overlays, with identical error output) | 345 | 345 | 345 |
  | Total wall time, the 555 that succeed | 366.0 s | 355.8 s | 450.1 s |
  | Mean peak RSS, all 900 | 182 MiB | 184 MiB | 210 MiB |
  | Highest peak RSS | 1456 MiB | 1537 MiB | 2969 MiB |

  Per scenario, against graphops.2: the median wall time ratio is 1.02 over
  the 202 succeeding scenarios that take 0.5 s or more (0.48 to 1.45; single
  renders on a shared host, so a few tens of percent are noise), the median
  peak RSS ratio 0.99, at most 1.29.

  Output against graphops.2: 886 scenarios are byte-identical, no exit status
  differs, and the 14 others differ only in YAML quoting or the order of
  keys, with equal decoded values. The share-past-def order differences of
  the previous sweep that came from function-argument vertices (17) are gone;
  so are the 77 of share-funcarg-chain.

  | Scenarios | What differs | Cause | Verdict |
  |---|---|---|---|
  | 10 (stack scenarios) | A YAML document in a ConfigMap: a value written `:8081` instead of `':8081'`; no key order differs | Upstream v0.18.0 `98bb3cf04` turns off `yamlgoccy` by default, so `yaml.Marshal` uses the yaml.v3 encoder again (`CUE_EXPERIMENT=yamlgoccy=1` gives graphops.2's bytes) | Upstream change; both forms are the same YAML string |
  | 3 (scenarios of two blueprints) | The field order of one blueprint's configuration | `perf/v0.18.0-share-past-def` in place of the fork's share-past-def; this branch without either gives graphops.2's order | Field order of sharing past a definition; see [above](#field-order-and-sharing-past-a-definition) |
  | 1 (one scenario) | Key order inside a JSON string kept in a Secret | graphops.2's own call memo | Correction; see [above](#field-order-of-a-call-result-taken-by-an-opened-copy) |

  Only the last one changes bytes inside a value (a Secret's string, so one
  rollout when moving from graphops.2). The others change the key order or
  quoting of rendered manifests, not the objects.

## Rendering the blueprints

[`sweep.sh`](sweep.sh) renders blueprint scenarios with two
or more `cue` binaries and compares exit status, output hash, wall time and
peak memory. A scenario is a blueprint directory, rendered as
`cue export -t mockContext=true . [testdata/<file>.cue] -e out --out json`
from that directory, once without an overlay and once with each
`testdata/*.cue` file. It renders one scenario at a time, takes the shared
lock for each render only, caps each at 4 GB without swap, stops at the
first render killed by a signal (an OOM kill), and refuses more than five
scenarios without `APPROVED=1`:

```sh
IK=<blueprint checkout> OUT=/path/outside/this/repo \
BINS="new=/path/to/cue-new old=/path/to/cue-old" bash fork/sweep.sh <kind>/<blueprint>:<overlay>.cue
APPROVED=1 IK=… OUT=… BINS=… bash fork/sweep.sh     # every scenario
```

Rendered outputs are GraphOps configuration: keep `OUT` outside this
repository; only aggregate numbers are recorded here, in
[Validation](#validation).

## Rebuilding

```sh
bash fork/rebuild.sh            # BRANCH=graphops/v0.18.0, PREV=BRANCH
```

`rebuild.sh` fetches `integration/v0.18.0` from `FOUNDATION_REPO` (default
`~/code/personal/cue`) into `refs/foundation/integration-v0.18.0`, creates a
twig worktree detached at `v0.18.0`, and then:

1. ports every commit of the integration branch: the fixes and performance
   changes with the fork's author, `Signed-off-by` and `Co-Authored-By`
   (dropping `Assisted-by`), the upstream commits with their author and
   message, the upstream commit they come from (found on `upstream/master`
   by author, date and subject: fetch `upstream` first) and the note in
   `NOTE` on what their port leaves out. It stops if a tree differs from the
   integration branch's at the same commit, or if an upstream commit has no
   note;
2. cherry-picks every commit of each branch in `patches.txt`, in order, with
   `git rerere` and the resolutions recorded in `rr-cache/`;
3. cherry-picks the commits of `PREV` that follow the patches: the
   combined-goldens commit and the `fork:` commits;
4. points `BRANCH` at the result and removes the worktree (`KEEP_WT=1` keeps
   it; `REBUILD_WT` chooses where it goes).

Run it when the integration branch or a patch branch changes, or with
`BRANCH=<new branch>` to build a candidate next to the line. With the same
inputs it gives the same tree; the commit hashes differ, as the committer
date does. To keep the previous line, rename it first and point `PREV` at
it, as for this build:

```sh
git branch -m graphops/v0.18.0 graphops/v0.18.0-candidate4
PREV=graphops/v0.18.0-candidate4 bash fork/rebuild.sh
```

When it stops at a conflict, it leaves the worktree in the middle of the
cherry-pick:

- A conflict in the recorded results of a `.txtar` file (the stats, or an
  error list): keep the side of the branch being built, regenerate the file
  with `CUE_UPDATE=1 go test ./internal/core/adt/ -run 'TestEvalV3/<dir>/<name>$'`,
  check that only the recorded results the commit itself changes differ, `git
  add` it and `git commit -C <commit> --cleanup=verbatim`. rerere records the
  resolution; run `rebuild.sh` again, and it copies the resolution to
  `rr-cache/`, to be committed with this file.
- A conflict in code: port the patch onto the new foundation, the same way,
  and update the patch branch if the change is needed on the tag as well.
- The combined-goldens commit no longer applies, or `go test ./...` fails
  after a rebuild: rebuild with `KEEP_WT=1`, drop that commit, regenerate with
  `CUE_UPDATE=1 go test ./internal/core/adt/ ./internal/core/compile/`, review
  every change, commit it with the same subject and a message listing each
  change with its reason, and point `PREV` at the result for the next run.

Then run `go vet ./...` and `go test ./...` on the result.

## Releases

Pushing a tag `v0.18.0-graphops.<n>` on this branch runs
`.github/workflows/release.yml`: it tests the fork and publishes a GitHub
release with `cue` binaries for Linux, macOS and Windows (amd64 and arm64),
named like upstream's (`cue_<tag>_<os>_<arch>.tar.gz`), plus
`sha256sums.txt`. `cue version` reports the tag. A manual run of the workflow
builds a snapshot without publishing.

Go programs that embed CUE use a release with a replace directive:

```
require cuelang.org/go v0.18.0
replace cuelang.org/go => github.com/graphops/cue v0.18.0-graphops.<n>
```

## Release notes

Draft for `v0.18.0-graphops.1`, the first release from this branch, against
`v0.18.0-rc.4-graphops.2`.

**Upstream base: v0.18.0.** The base moves from v0.18.0-rc.4 to the v0.18.0
release (27 upstream commits: YAML 1.1 decoding and encoding fixes and
bounded merge-key aliases, `yamlgoccy` off by default, linear-time TOML
decoding, the error of the field that fails a disjunct and no typo check of
the fields a disjunct skipped, `cue lsp` accepting older flags, `let X =
self` export, compact spacing in `cue fmt`), plus seven upstream `master`
commits: `6b239f555`, `cd43cfbec`, `8c4aa86de` (a validator's errors no longer
leak into the next one), `7d575eaaf`, `f6483025c` (#4598, closures of a
pattern that differ only by label no longer share a call result),
`8fbe49d7d`, `582825123` (#4595, a field a comprehension adds keeps its value
when the struct is iterated first).

**Fixes.** The 21 fixes of the [Foundation](#foundation), as the
dusk-to-dawn branches have them on 2026-11-25. New in kind since
graphops.2: walk-try-escapes, disjunct-cycle-skip (a disjunction no longer
silently dropped for a field looked up from within a value it unifies),
closed-list-disjunct (no spurious "field not allowed" for a list
comprehension under a disjunction), share-late-conjunct (#4524), and
test-eq-error in the test harness. The others replace graphops.2's own
versions of the same fixes with the versions reworked for upstream:
call-arg-cycle, deferred-completion-disjunct-scope, disjunct-pushdown-notify,
comprehension-dependency (was "fail the fields declared by a failing
comprehension"), export-let, fmt-alias-form, let-comprehension-pushdown,
lookup-error-leak, nested-disjunction-errors, pushdown-notify-reference
(#4550), running-producer, try-speculative (#4486; was try-finalize,
try-share-disjunction and try-target-probe), share-disjunction-in-progress,
share-funcarg-chain, struct-comprehension-top (#4577; was "a
comprehension-only struct literal is a struct next to top") and
toposort-stable-sort. Behaviour changes to expect: configurations that
graphops.2 accepted because a conjunct, a disjunction or a constraint was
silently lost may now fail with an error that names the real conflict.

**Performance.** call-constraint-replay and call-memo replace the fork's call
memo: calls are keyed by what their arguments read, and the parameter
constraints of a call are added once per activation. share-past-def replaces
the fork's version; without it ten stack scenarios take twice the time
and memory. See [Validation](#validation) for time and memory on the
blueprints.

**GraphOps-only.** Unchanged in content: `json.MarshalSorted` /
`yaml.MarshalSorted`, `struct.Data`, the shallow build API,
buildexpr-closedness (upstream `0beee1028`, `42875a302`), call-closedness-path,
try-closed-field, embed-root; late-conjunct gains its third commit, which
reports a conjunct that reaches a field after its disjunctions were processed.

**Visible output changes.** In the blueprint sweep, 14 of 900 scenarios
render other bytes than with graphops.2, all with equal values: one
`json.Marshal` key order inside a Secret (a correction; see
[Field order](#field-order-of-a-call-result-taken-by-an-opened-copy)), the
field order of one blueprint's configuration in 3, where share-past-def and
graphops.2's version of it differ, and YAML quoting in 10, from upstream
v0.18.0's YAML encoder. share-funcarg-chain and share-past-def keep the order
of fields of function arguments (see
[Field order and sharing past a definition](#field-order-and-sharing-past-a-definition)).
Configurations that hash rendered manifests, or keep them in a repository,
see one diff.

**Patch list** (`patches.txt`, applied on the foundation in this order):
`feat/v0.18.0-marshal-sorted`, `fix/v0.18.0-struct-data-incomplete`,
`feat/v0.18.0-build-shallow`, `fix/v0.18.0-buildexpr-closedness`,
`fix/v0.18.0-call-closedness-path`, `fix/v0.18.0-late-conjunct`,
`fix/v0.18.0-try-closed-field`, `fix/v0.18.0-embed-root`.

## Build and test

```sh
git checkout graphops/v0.18.0
go build -o cue ./cmd/cue
go test ./...
```
