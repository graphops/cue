# GraphOps fork of CUE v0.18.0-rc.4

Evaluator fixes and additions GraphOps needs to run its configurations on CUE
v0.18.0-rc.4, until equivalent changes land upstream. Nothing here has been
reported or proposed upstream yet; each fix is meant to become an upstream
issue and change once its analysis is reduced to a self-contained case.

This branch starts at the `v0.18.0-rc.4` tag; the release
`v0.18.0-rc.4-graphops.1` is this branch with `fix/rc4-late-conjunct`,
`fix/rc4-validator-error-leak` and `fix/rc4-lookup-error-leak` applied. See `GRAPHOPS.md` on `main` for
the fork's branch conventions. It carries over every fix and addition of
`graphops/v0.18.0-rc.3` that upstream has not made unnecessary (none of
them); see [Moving from v0.18.0-rc.3](#moving-from-v0180-rc3).

## Fixes

Each fix also exists on its own branch, isolated on the `v0.18.0-rc.4` tag.

| Branch | What it fixes | Confidence |
|---|---|---|
| `fix/rc4-uninit-parent-state` | Panic "vertex … not initialized": a bare parent state, from a reclaimed and revived vertex | Medium-high |
| `fix/rc4-let-pushdown` | Silent value loss: a `let` in a comprehension body blocks its pushdown | High |
| `fix/rc4-try-finalize` | Silent constraint loss or spurious cycle: a try clause pre-evaluates its expression by finalizing an inline vertex, which may finalize the fields its body yields into before it yields (`try X = x? {x: a: <=2}` exported `x.a: 5`; `{x: b: 1}` was a cyclic reference; a dynamic field was dropped). While the clause pre-evaluates, finalizing a vertex it is a parent task of is lowered to attemptOnly, and the field set is not frozen; other clauses still report their cycles. Since `339485ddf` (v0.17.0-alpha.2) | High |
| `fix/rc4-ancestor-flags` | Ancestor completion flags attributed while parent tasks are pending | Hardening, no known impact |
| `fix/rc4-try-target-probe` | Spurious cycle from a try nested in another try's body | High |
| `fix/rc4-ready-parent-completion` | Non-concrete value: a pushed-down arc completes before its comprehension has run | High |
| `fix/rc4-running-producer` | Silent constraint loss: a try body discarded because the arc type it reads was frozen while its producer comprehension was running | Medium-high |
| `fix/rc4-call-memo-copies` | Performance: every copy of a function result re-ran its nested calls | Medium-high |
| `fix/rc4-call-arg-cycle` | Spurious structural cycle: a call inherited from a cycle its arguments do not reach was postponed until it became a structural cycle | Medium-high |
| `fix/rc4-call-memo-bindings` | Performance: calls on comprehension variables re-ran in every copy; also a wrong result when a pattern passes its label alias to a function. Based on `fix/rc4-call-memo-copies` | Medium |
| `fix/rc4-pushdown-notify` | Silent value loss: a reference copied a struct while a pushed-down comprehension targeting it was still running, so the fields the comprehension added afterwards never reached the referring node | Medium-high |
| `fix/rc4-arctype-validate` | Incomplete value, order-dependent: iterating a struct before it is evaluated validated a comprehension-declared arc while its reference was unresolved, fixing it to `_` ("undefined as v is incomplete (type _)"; the arc's fields lost) | Medium-high |
| `fix/rc4-export-let` | Panic "unreachable" (or a stack overflow, or a silently detached expression) printing non-concrete values: a `let X = self` referenced outside the struct declaring it, as through `#T: #S & {}`, was expanded in place, and comprehension clause expressions were exported one environment too deep. Affects `cue eval` and `Value.Syntax`, not evaluation or `cue export` | Medium-high |
| `fix/rc4-buildexpr-closedness` | Silent constraint loss: a conjunction built with `Context.BuildExpr`, as by `cue eval -e '#S & d'`, skipped definition closedness, because its value was finalized in a second evaluation context that suppresses typo checks | High |
| `fix/rc4-nested-disjunction-errors` | Lost diagnostics: when every disjunct failed only through a nested disjunction (e.g. each reads a field whose own disjunction failed), the error was a bare "empty disjunction" without any cause | High |
| `fix/rc4-fmt-alias-form` | `cue fmt` wrote the single postfix alias as `~(X)` for files whose language version predates it (below v0.17.0, which cannot parse it); it now keeps `~X` there | High |
| `fix/rc4-struct-data-incomplete` | `struct.Data` (an addition below) turned an argument that was only incomplete, such as one reading an optional field, into a permanent error, failing evaluation of a field nobody asked to be concrete; it now keeps the argument's error code, as `json.Unmarshal(json.Marshal(v))` does. Based on the `struct.Data` addition | High |
| `fix/rc4-share-disjunction-safe` | Spurious "cyclic reference to field" or lost fields, a regression in v0.18.0-rc.2: upstream `082c52c57` finalizes the vertex a field shares when the field meets a disjunction, to keep sharing. When the field is only evaluated in part, from within an evaluation the vertex depends on (a comprehension of the vertex, or of a vertex it refers to, reading the field), that evaluation was forced before it added its fields. The vertex is now finalized early only while the sharing field is itself being finalized. The self-reference half of the rc.2 fix is upstream since `4232b8d36` | High |
| `fix/rc4-share-funcarg-chain` | Spurious structural cycle, upstream since functions exist: a function applied, through a wrapper, to the result of an earlier call of the same call site, without any definition. A reference to a parameter that structure shares its argument was dereferenced to the argument's value, evaluated with the callee body's cycle-reference chain instead of the caller's; it now stops at the parameter, whose argument conjunct carries the caller's chain. With the sharing addition below, it also covers the same cycle with a definition on the argument (test `cue/testdata/functions/sharedefs_reapply.txtar`, carried from the former `fix/rc2-share-funcarg-cycle`) | Medium-high |
| `fix/rc4-try-share-disjunction` | Silent constraint loss: a try clause pre-evaluates its body in an inline vertex, which shared the field the body starts with and so resolved a disjunction of that field before the clause yielded into it; what the clause yielded later reached a node that had already resolved its disjunction and was dropped without an error (`x: r: "4k" \| "32k"`, `x: r: "32k"`, `try _ = x.r? {x: r: "zz"}` exported `r: "32k"`; a default `*"4k" \| "32k"` gave `"4k"`). The inline vertex of a try body does not share a vertex that has a pre-evaluating parent task; it evaluates its conjuncts in a copy. Reached since `aab026e95` with a definition; without one it is older. Not covered: the field reached through another field that shares it, or through a disjunction of an ancestor. Based on `fix/rc4-try-finalize` | High |
| `fix/rc4-comprehension-error-loss` | Silent value loss: a field declared next to a struct-level comprehension (such as `x: [...]`) kept its declared value when the comprehension's clauses failed, because the error was recorded only on the enclosing struct; a reference evaluated before the struct exported `[]` with exit 0. The error is now added as a conjunct of each field the body or else clause declares, so embeddings and loops over the field see it too. Incomplete and else cases since `339485ddf` (v0.17.0-alpha.2); a fatal guard without else since `748a68589` (v0.5.0-alpha.1) | High |
| `fix/rc4-call-closedness-path` | Lost diagnostics, a regression in v0.18.0-rc.2: upstream `7bf42f62f` reports an error arising in a function call at the field holding the call, but dropped the error's path within the argument, so a closedness error below an argument read only "field not allowed", and distinct failures at the same call were deduplicated into one. An error below an argument is now wrapped in an error naming its path from the parameter (`w: cfg.extra: field not allowed`), whatever its kind, also when it wraps another error such as a failed validator | High |
| `fix/rc4-deferred-disjunct-scope` | Spurious "field not allowed" and silently accepted invalid values, a regression in v0.18.0-rc.2: upstream `b68cd7163` completes a field of an inline struct or let that a task read too early with a task recorded by `rootedTaskNode`, which searched the whole task stack. Inside a disjunct, that recorded the completion outside of it: an error of a discarded disjunct failed the referrer, and a let's error landed in another disjunct, accepting an invalid one. The search now only considers the tasks the innermost disjunct started (the task stack depth recorded in its overlay frame) | High |
| `fix/rc4-disjunct-pushdown-notify` | Silent value loss, a regression in v0.18.0-rc.1: a struct whose comprehension declares fields, unified with a disjunction (`out: ({...} | int) & {a: 1, for _, v in out {b: v}}`, or `out: x & (#M | int)` with the comprehension in `x`), lacked those fields when the comprehension had not run before the disjunct was cloned. The arcs cloned for a disjunct lost their parent tasks, and upstream `d47e4cee8` settles a pending arc without active parent tasks as absent; since `e147bfc48`, a guard testing such a field for absence also finds it absent instead of reporting a cycle. The cloned arcs now point at the cloned comprehension. Not covered: a reference copying a field that a comprehension adds to later, when the referring field meets a disjunction (`out: x.args & ({...} | int)`, `x: {args: a: 1, if out.a == 1 {args: b: 2}}`), still loses those fields (since `339485ddf`) | High |
| `fix/rc4-try-closed-field` | Spurious "undefined field", order dependent, upstream since try exists: a `?`-marked selection of a field a closed struct does not declare (`d.y?`) failed with a permanent error instead of discarding the try, but only if the lookup found the struct already evaluated, as closedness is recorded once its conjuncts are processed. Moving the field below the try made it pass; `339485ddf` (v0.17.0-alpha.2) made more orders pass by running comprehensions earlier. Selecting through a disjunction evaluates its disjuncts first, so `x.k.y?` on a default branch that lacks `y` always failed, even if another branch declares it. Such a field is now absent, as in an open struct. This changes behaviour upstream's `closedness` cases in `try.txtar` recorded, in line with proposal 4019: a misspelled `?` selection is discarded in every order, not only in some. Without `?`, the lookup still fails; so do a closed list index and an error in the selected value | Medium-high |
| `fix/rc4-toposort-stable-sort` | Field order depending on where the module cache is, upstream since `65a15e2b6` (v0.11.0-alpha.5), with a second cause since `00349c26d` (v0.17.0-alpha.2): the structs contributing to a vertex are sorted by position, and positions in different files compare by file name, so a dependency's files in the module cache sorted before or after the main module's depending on the cache's directory. Evaluating one module with byte-identical dependencies in two cache locations could give two field orders in `cue export` and `json.Marshal`. The sort was unstable, so roots at equal positions (all the structs reached through one reference) were permuted by the roots of other files around them; and a run of roots from one comprehension firing could continue into another file, losing the iteration order. The roots are now sorted stably and a run ends at the end of a file, so the order of files has no effect. The field order of such structs changes once, to the order the evaluator added them in. The commit applies unchanged on `v0.18.0-rc.4` | High |
| `fix/rc4-embed-root` | Files outside the module embedded through symbolic links, upstream since embedding exists (v0.10.0): `@embed(file=...)` and `@embed(glob=...)` checked their paths only lexically and read them through the OS file system, which follows links, so a link within the module could embed any file or directory on the machine. Every path an embedding reads is now resolved with `os.Root` on the module root first; one resolving outside of it, through a link leading out or an absolute link, fails the embedding with an error naming the argument and the path, and so does a glob reading such a directory while matching. Relative links within the module keep working, as do overlay-only files; `load.Config.FS` file systems are used as given. Absolute links are refused even when they lead within the module. The commit applies unchanged on `v0.18.0-rc.4` | High |
| `fix/rc4-late-conjunct` | Silently accepted invalid configurations, upstream since conjuncts could be added to an arc after it was read (v0.16.1, v0.17.1 and v0.18.0-rc.4 accept the examples): a finalized arc takes no more conjuncts, so `insertConjunct` dropped one added later without a trace (a comprehension that waits for another one, whose guard read the arc, such as `n: a: enabled: false` with `for _, _ in r {n: a: enabled: true}` after `for _, _ in r {if n.a == _\|_ {r: x: 2}}`), and a conflict, a field a closed struct does not allow, or a narrowing (a type that becomes a value, a default that is not selected, an optional field that becomes regular) was hidden, after the tasks that read the arc had acted on its value. The dropped conjuncts are recorded per arc and checked when the node that inserted them has completed its arcs, before it collects their errors: one the arc evidently has is skipped, the others are unified with the arc's own conjuncts (so closedness is judged as for the arc) and the result is compared with the arc by what evaluation gives (validators and function types are compared after finalizing), reporting a conflict, a new field, or a changed value at the arc's path. A value that agrees is no error; an incomplete result is not one either. Behaviour change: a late conjunct that narrows an arc is now an error (`functions/tighten.txtar` has two definitions a comprehension narrows from `number` to `int` after a call read them). Cost is that of the dropped conjunct, linear in the size of the arc | High |
| `fix/rc4-validator-error-leak` | Wrong results, a regression in v0.18.0-rc.2 (`56d5c165e` checks the validators of a scalar while its conjuncts are inserted): a validator that records the errors of the schemas it tried (`matchN` does, besides returning its failure) left them in the evaluation context, and the next operation of the same evaluation saw an error where it asked for a boolean, so equal scalars compared unequal. Three copies of `matchN(1, [int]) \| null` evaluated to `matchN(1, [int])`, four or two next to a concrete `null` were an error ("conflicting values null and null"), and a satisfied `matchN` that ran after a failed one listed itself as failed. `runChecks` now runs each validator with a context free of errors and gives the node what it recorded together with the error it returned, only when the context is the node's own; a validator that records an error without returning one (a builtin called with an argument of the wrong type) has failed, and what it recorded is its error, so `strings.MinRunes("x") & "abc"` is still reported. The causes of a failed `matchN` are listed once, under the validator that failed. Branch `fix/rc4-validator-error-leak-v2` | High |
| `fix/rc4-lookup-error-leak` | Wrong results, upstream since v0.17 (v0.17.1 and v0.18.0-rc.4 have it): a reference to a required field records the incomplete error "required field missing" in the context and returns the field, for the node of the reference to take when its task ends. Inserting the scalar the field holds compared it with an equal scalar already in the node through `BinOpBool`, which asks the context for the boolean result and gets the pending error instead, so equal scalars compared unequal and `a!: "x"`, `b: a`, `b: "x"` reported `conflicting values "x" and "x"` (every scalar kind, either order, through a selector, a definition, a pattern, a list element, a let and in a disjunct). The incomplete error became a final one, which showed wherever the two are told apart: a comprehension guard on `b == "x"` failed instead of staying unresolved, `b \| *"d"` took the default, `b != _\|_` was false. `BinOpBool`, the one reader that did not drain the context first, now runs the comparison with a context free of errors and hands back what it held and what the comparison recorded, for the frame that owns them. The reference, the writer, is unchanged | High (reader fix; under independent review when added) |

## Additions

Not bug fixes: features configurations built on this fork rely on. They exist
only on this branch and, with the fix based on each, on the fix branches noted
above, or on the `feat/` branch a row names.

| Addition | What it adds | Test |
|---|---|---|
| `struct.Data(v)` | `v` as concrete data (regular fields, defaults resolved, no closedness, definitions, hidden or optional fields), built as fresh CUE literals. An in-CUE replacement for `json.Unmarshal(json.Marshal(v))`, about 2.7× cheaper, which keeps CUE kinds such as bytes. An incomplete argument makes the call incomplete, as with the JSON round trip. | `pkg/struct/testdata/data.txtar`, `cmd/cue/cmd/testdata/script/struct_data_incomplete.txtar` |
| Sharing past a repeated definition | Structure sharing is kept when a conjunct refers to a definition the shared value already applied (plainly, or opened for an opened repeat); the repeated conjunct is held and applied as usual if the node is unshared. A schema-checked value can then be passed to many places that also declare the schema. Upstream keeps sharing since `1c287ec21` when the shared value holds the definition as a direct conjunct; this extends it to definitions applied any other way (such as through a field of an opened definition), to a definition arriving before the value, and to opened definitions. Not applied to a plain definition re-closing an opened reference (`x... & #D`), nor within a function argument, where it turned a nested call into a spurious structural cycle (a second commit, test `cue/testdata/functions/sharedefs_argcall.txtar`). On branch `feat/rc4-share-past-def`. | `cue/testdata/eval/sharedefs.txtar`, `cue/testdata/functions/sharedefs_reapply.txtar`, `cue/testdata/functions/sharedefs_argcall.txtar` |
| `json.MarshalSorted(v)`, `yaml.MarshalSorted(v)`, `Value.MarshalJSONSorted` | `json.Marshal` and `yaml.Marshal` with the fields of every struct ordered by key, at every depth, comparing the unquoted keys byte by byte, so that content hashed from the result, as for the name of an immutable ConfigMap, does not change when a declaration moves. List order, the encoding of keys and values, and errors are those of the plain functions. On branch `feat/rc4-marshal-sorted`. | `pkg/encoding/json/testdata/marshalsorted.txtar`, `pkg/encoding/yaml/testdata/marshalsorted.txtar`, `cmd/cue/cmd/testdata/script/marshal_sorted_incomplete.txtar` |
| `Context.BuildInstanceShallow`, `Value.LookupPathShallow`, `Value.ShallowSelectors` | Go API. Reading a few fields of a large package without finalizing all of it: the build computes only the package's fields, a shallow lookup evaluates only the value it reaches, and `ShallowSelectors` lists a value's fields, definitions, hidden, optional and required ones included, without evaluating them. Errors in fields nobody looked up are not reported. A value looked up shallowly is the value a full build gives, but its syntax may inline a reference to a field outside itself that a full build keeps. On branch `feat/rc4-build-shallow`. | `cue/shallow_test.go` |

## Moving from v0.18.0-rc.3

Every fix and addition of `graphops/v0.18.0-rc.3` (release
`v0.18.0-rc.3-graphops.5`) was checked on plain v0.18.0-rc.4 by applying only
its tests, and the fix or addition was then ported onto the tag. Plain
v0.18.0-rc.4 passes `go test ./...`, and every regression test below still
fails on it, so nothing was dropped. v0.18.0-rc.4's commits to the evaluator
(`8ea869553` unify an inline arc before a lookup yields on it, `0292b1a41` do
not restart a sharing vertex whose target is done, `874b0b84b` stop skipping
comprehensions on cyclic arcs, `91467a4ba` ignore a parent's conjuncts from
another context, `8063ca2f2` close a `__reclose` literal by a field not yet
evaluated, `182516776` rename `Default` to `ConcreteDefault`) do not cover any
of them.

Dropped: none.

Adapted:

- `feat/rc4-share-past-def` (the sharing addition): its regression test
  `eval/sharedefs.txtar` still fails on plain v0.18.0-rc.4, so the addition is
  carried. It also carries a second commit, with the regression test
  `functions/sharedefs_argcall.txtar`: kept sharing past a definition turned a
  nested call into a spurious structural cycle. A parameter field sharing the
  argument of a call was not unshared by the parameter constraint, so a lookup
  through it reached a field of the entry whose conjuncts hold a call, under
  the callee's cycle-reference chain instead of the one the argument pins. The
  node is now left unshared (as before the addition) when it, or an ancestor,
  is a parameter whose argument pins the caller's chain (`CycleInfo.IsFuncArg`;
  the mark is on the parameter's conjunct, not on those of the fields of an
  argument that is a struct). The same cycle without any definition exists on
  plain v0.18.0-rc.2, rc.3 and rc.4: it is upstream's and is not fixed here.
- `struct.Data` (in `fix/rc4-struct-data-incomplete`): calls
  `Vertex.ConcreteDefault`, the name `Vertex.Default` has in v0.18.0-rc.4.
- `fix/rc4-try-share-disjunction`: new on this base. A try body pre-evaluates
  in an inline vertex that shared the field it reads, which resolved a
  disjunction of the field before the clause yielded into it, so a conflicting
  or defaulted yield was lost (`x: r: "4k" | "32k"`, `x: r: "32k"`,
  `try _ = x.r? {x: r: "zz"}` exported `r: "32k"`). Plain v0.18.0-rc.4 leaves
  it. Based on `fix/rc4-try-finalize`.
- `fix/rc4-running-producer`, `fix/rc4-call-arg-cycle`,
  `fix/rc4-call-memo-bindings`, `fix/rc4-share-disjunction-safe`,
  `fix/rc4-call-closedness-path`, `fix/rc4-try-finalize`: each branch is based
  on the plain tag, so the code ports without change, but on this branch they
  stack on the fixes before them (the try clause's `comp`, the call memo
  helpers in `export_test.go`, the first versions of the commits their
  revisions replace); the commits here are the ones of the v0.18.0-rc.3
  branch, in its order, with the code unchanged.
- `fix/rc4-call-memo-copies`, `fix/rc4-call-memo-bindings`,
  `fix/rc4-try-closed-field`: the `internal/core/compile/testdata/sync`
  mirrors of their test files, which were missing on the v0.18.0-rc.3 branches,
  are generated (`CUE_UPDATE=1 go test ./internal/core/compile -run 'TestCompile$'`)
  and carried. Every port carries its mirror, and `CUE_UPDATE=diff go test`
  is clean on each port and on this branch.
- Golden files only (evaluation counters of the tests the fixes change, and
  `builtins/reclose.txtar`, which v0.18.0-rc.4 extends): the ports that touch
  `native.txtar`, `tighten.txtar`, `reclose.txtar`, `sharing.txtar` and the
  stats of the new tests. The goldens that depend on several fixes together
  are updated in one commit, as before.

New on this base (not in `v0.18.0-rc.3-graphops.5`):

- `fix/rc4-late-conjunct`: a conjunct added to an arc after it was finalized
  was dropped without a trace, so a conflict with the arc was not reported. An
  earlier version for v0.18.0-rc.3 was never released; this one is rebuilt
  directly on v0.18.0-rc.4. The golden changes that depend on the other fixes of this
  branch are one commit on the release branch: the two new errors of
  `functions/tighten.txtar`, and `comprehensions/try.txtar`, where
  `tryOwnFields.guardAfterTry.x` (an invalid program) is now reported as
  "adding field b not allowed as field set was already referenced" instead of
  as an incomplete cyclic reference.
  Plain v0.18.0-rc.4 accepts the examples of the test
  `comprehensions/lateconjunct.txtar`.
- `fix/rc4-validator-error-leak`: the errors a validator recorded stayed in
  the context and made an unrelated operation fail, so a disjunction of a
  validator and a scalar lost the scalar. Also a regression of v0.18.0-rc.2;
  plain v0.18.0-rc.4 still has it (`disjunctions/validatorscalar.txtar`).
  `fix/rc4-validator-error-leak-v2` is the revised fix, with the handling of a
  validator that records an error without returning one; the release carries it
  as one commit.
- `fix/rc4-lookup-error-leak`: a scalar comparison consumed the incomplete
  error a required-field reference had recorded for its node, so equal scalars
  conflicted. Present in plain v0.18.0-rc.4 and v0.17.1
  (`references/requiredscalar.txtar`); it composes with the validator fix
  without a golden change.

## Earlier move: from v0.18.0-rc.2 to v0.18.0-rc.3

Every fix of `graphops/v0.18.0-rc.2` was checked on plain v0.18.0-rc.3 by
applying only its tests, then reapplied where its regression test still
fails. Plain v0.18.0-rc.3 passes `go test ./...`.

Dropped, because v0.18.0-rc.3 fixes them upstream:

- `fix/rc2-else-pushdown`: upstream `154d2ce09` makes the same change
  ("push down the else clause of a comprehension").
- `fix/rc2-lookup-settle-arc`: its test passes on plain v0.18.0-rc.3.
  Upstream `e147bfc48` stops lookups from forcing pushed-down comprehensions
  at all, which replaces the rc.2 shortcut this fix corrected.
- Half of `fix/rc2-share-disjunction-safe`: re-sharing what the finalized
  field shares, which kept a definition from being dropped as a
  self-reference. Upstream `4232b8d36` compares a recorded reference arc by
  identity, which fixes that at its source; the re-sharing also skipped the
  links upstream's new `sharesOpened` (`55ea1639a`) reads, closing a node
  that an opened field in the chain had opened (`explicitopen.txtar`,
  `forwardShared`). Its tests are kept and pass.

Not carried, as on `graphops/v0.18.0-rc.2`: `fix/rc2-share-disjunction-finalize`
(a revert of `082c52c57` that rc.2-graphops.2 reverted again) and the code of
`fix/rc2-share-funcarg-cycle` (subsumed by `fix/rc2-share-funcarg-chain`; its
test is carried as its own commit).

Every other fix and both additions are carried. Their regression tests fail on
plain v0.18.0-rc.3 without them. These needed adapting:

- `fix/rc3-let-pushdown`: the code is unchanged. Upstream `e147bfc48` lets a
  lookup decide that a field missing from a node whose only remaining tasks
  are pushed-down comprehensions cannot exist. With the fix, a comprehension
  whose body holds lets is pushed down, so `letCycleSelfKeyword2` in
  `pushdown.txtar` reports "undefined field: hosts" instead of "cannot range
  over s.hosts (incomplete)", still as an incomplete error. Upstream made the
  same change to the `let this = self` case in `nested2.txtar`.
- `fix/rc3-call-arg-cycle`: upstream `897d511cf` moved the call check ahead of
  the deferral in `conjunct.go`; the deferral now tests whether the call's
  arguments reach the cycle within upstream's structure.
- `fix/rc3-share-disjunction-safe`: reduced to its in-progress check
  (since replaced, see below).
- `fix/rc3-share-funcarg-chain`: placed next to upstream's new `sharesOpened`;
  the code is unchanged.
- `fix/rc3-export-let`, `fix/rc3-call-memo-copies`,
  `fix/rc3-call-memo-bindings`, `fix/rc3-call-closedness-path`,
  `fix/rc3-comprehension-error-loss`: golden files only (v0.18.0-rc.3 numbers
  new let names sequentially, reports some errors at a more precise path, and
  moved evaluation counters).

On this branch, the goldens that depend on several fixes together are
updated in one commit.

## Revised while preparing upstream changes

Batch 1 of the upstream preparation re-derived eight fixes from first
principles, as single commits on `cjorge-graphops/cue` branches based on
upstream master, with issue drafts on this repository's branch
`upstream/rc3-batch1` (`fork/upstream/`). Where that produced a better fix,
it replaced the one here, as commits on top of this branch and of the
`fix/rc3-` branch; render parity with `v0.18.0-rc.3-graphops.1` holds:

- `fix/rc3-share-disjunction-safe`: the walk for an evaluation in
  progress missed one in a vertex the shared one refers to (`x: y`); now
  a condition on the sharing field (`fix/share-disjunction-in-progress`).
- `fix/rc3-call-closedness-path`: the same output with a third of the
  code and no new error type (`fix/call-error-argument-path`). **Not
  proposed upstream; fork-only for now.** Upstream is reworking how the
  inner paths of wrapped errors are reported (#2782), which will likely fix
  this or change how it should be fixed. Revisit when that work lands.
- `fix/rc3-try-finalize`: the exemption covered every clause of a
  comprehension with a try clause, hiding the cycle of a later if clause,
  and a dynamic field was still rejected; now limited to the try clause's
  pre-evaluation (`fix/try-own-arcs-finalize`).
- `fix/rc3-deferred-disjunct-scope`: the same results without an
  unreachable fallback that loses a let's error when forced
  (`fix/deferred-completion-disjunct-scope`).
- `fix/rc3-comprehension-error-loss`: the error recorded in the fields'
  state was missed by an embedding of the field; now added as a conjunct
  (`fix/comprehension-error-fields`).

`let-pushdown`, `pushdown-notify` and `buildexpr-closedness` kept their
code; their upstream versions only change comments and tests.

## Releases

Pushing a tag `v0.18.0-rc.4-graphops.<n>` on this branch runs
`.github/workflows/release.yml`: it tests the fork and publishes a GitHub
release with `cue` binaries for Linux, macOS and Windows (amd64 and arm64),
named like upstream's (`cue_<tag>_<os>_<arch>.tar.gz`), plus `sha256sums.txt`.
`cue version` reports the tag. Increment `<n>` for each release from this
branch; a manual run of the workflow builds a snapshot without publishing.

Go programs that embed CUE use a release with a replace directive:

```
require cuelang.org/go v0.18.0-rc.4
replace cuelang.org/go => github.com/graphops/cue v0.18.0-rc.4-graphops.1
```

## Build and test

```sh
git checkout graphops/v0.18.0-rc.4
go build -o cue ./cmd/cue
go test ./...
```

Every fix passes `go test ./...`, has a regression test that fails without
it, and names the upstream commit that introduced the bug where bisection
found one. The detailed analyses are kept privately, because their original
reproducers come from private configuration.
