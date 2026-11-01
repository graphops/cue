# Side findings: to be tackled later

Three evaluator bugs found by chance on 2026-11-01, while reducing the
[share-disjunction-safe](../share-disjunction-safe/) reproducer. None is fixed
by the fork, and none affects GraphOps renders today. The cases are in
[`cases.txtar`](cases.txtar). Each is run with `cue export <file>`.

What is known so far comes from running each case on plain builds of v0.16.1,
v0.17.1, v0.18.0-rc.1 and v0.18.0-rc.3, on `graphops/v0.18.0-rc.3`, and on a
few hand-made variants. Neither bisection nor root-cause analysis has been
done yet.

All three share one shape with share-disjunction-safe, but none is caused by
the same commit: each behaves the same on rc.3 with `082c52c57` reverted, and
with the upstream fix prepared for it (`fix/share-disjunction-in-progress`).
The shape: a struct `x`
reads from `out`, `out: x`, and `out` is also unified with a disjunction
containing an open definition (`#M: {...}`).

## u5: `out` silently loses a field of `x` (priority: high)

```cue
#M: {...}
x: {
	if true {a: 1}
	for k, v in out {b: v}
}
out: x
out: #M | int
```

| Build | Result |
|---|---|
| v0.16.1 | error, and printing the error panics (`%!v(PANIC=String method: …)`) |
| v0.17.1 | correct: `x` and `out` are both `{a: 1, b: 1}` |
| v0.18.0-rc.1, rc.3, fork | **wrong, exit 0**: `x: {a: 1, b: 1}`, `out: {a: 1}` |

- **Why it is wrong:** `out: x` means `out` has at least every field of `x`.
- **Triggers:** correct without the disjunction (`out: #M`) and when the loop
  reads `x` instead of `out`. Still wrong with `a: 1` written without the `if`.
- **Why it is the priority:** a silent wrong value, a regression in
  v0.18.0-rc.1.
- **Next:**
  1. Bisect `v0.17.1..v0.18.0-rc.1`.
  2. Find the root cause in a debug build.
  3. Check how it relates to share-disjunction-safe (upstream `082c52c57`
     is in rc.2, not rc.1, so it is probably a different commit).

## p4: spurious "field not allowed" against an open definition

```cue
#A: #M | int
#M: {...}
x: {
	a: 1
	b: out.a
}
out: x
out: #A
```

| Build | Result |
|---|---|
| v0.16.1 | correct: `x` and `out` are both `{a: 1, b: 1}` |
| v0.17.1, v0.18.0-rc.1, rc.3, fork | `out.a: field not allowed`, `out.b: field not allowed` |

- **Why it is wrong:** `#M: {...}` is open and allows any field.
- **Triggers:** the error needs all three of the following, otherwise the
  result is correct:
  - the disjunction comes through an alias (`#A`), not `#M | int` written
    inline;
  - `out` has the disjunction at all (not `out: #M`);
  - `b` reads through `out` (not `x.a`, and not `b: 1`).

  The same narrow trigger as share-disjunction-safe.
- **Next:**
  1. Bisect `v0.16.1..v0.17.1`.
  2. Find the root cause in a debug build. Likely around structure sharing
     and closedness (`internal/core/adt/share.go`, re-closing opened shared
     vertices).

## b5: `cue export` panics "not properly initialized"

```cue
#M: {...}
y: x.sub.d
x: sub: {
	if out.sub != _|_ {d: 2}
}
out: x
out: #M | string
```

| Build | Result |
|---|---|
| v0.16.1 | error: `out: conflicting values …` |
| v0.17.1, v0.18.0-rc.1, rc.3, fork | `cue export` panics: `not properly initialized (state: unprocessed, value: <nil>)` |

- **Where it fails:** in JSON encoding (`cue.makeValue` ←
  `structValue.At` ← `structValue.appendJSON`), not in evaluation. `cue eval`
  on the same file prints `y: 2` and `out: sub: d: 2`. So evaluation leaves an
  arc unprocessed, and export reads it.
- **Variants:** without `y`, or without the disjunction, rc.3 reports `out.sub:
  cyclic reference to field d`.
- **Open question:** what the correct result is. The guard tests `out.sub`,
  the struct it adds to, so a cycle error may be the intended answer. A panic
  is a bug either way.
- **Next:**
  1. Bisect when the panic started (`v0.16.1..v0.17.1`).
  2. Find which arc is left unprocessed and why.
  3. Decide the expected result before writing the reproducer's
     expectation.

## When picking these up

Follow the batch-1 process (see [`../README.md`](../README.md)):

- a `repro.txtar` that fails on current upstream;
- a bisected introducing commit, confirmed by building it and its parent;
- a root cause confirmed in a debug build;
- a fix on a `fix/<base>-<topic>` branch, validated with the blueprint parity
  scan.

Re-check each case on the current upstream tag first; they may have been
fixed since.

## More findings from preparing batch 1 for upstream

Found by the agents re-deriving the batch-1 fixes. None is fixed by the fork or
by the prepared upstream fixes. Verified by hand where noted. The others are
as reported, and should be re-checked before work starts on them.

- **Constraint lost behind a guard needing the final value (verified).**
  `x: a: 5` plus `if json.Marshal(x) != "" {x: a: <=2}`: v0.16.1 reports
  `x.a: invalid value 5 (out of bound <=2)`, while v0.17.1, rc.3 and the fork
  export `{"x":{"a":5}}`. A regression since v0.17. Likely the 339485ddf
  pattern: `insertConjunct` drops conjuncts into a finalized vertex without
  an error. Making that drop an error would catch the whole class, but 28
  existing drops in the test suite would need review first (let-pushdown
  agent).
- **Field lost through a selected reference (verified, old).**
  `a: x.y & {z: 1}`, `x: y: on: true`, `if a.on {x: w}`, `w: y: v: 1`: `a`
  lacks `v: 1` although `x.y` has it, on v0.16.1, rc.3 and the fork. Not a
  regression. Found while reducing pushdown-notify.
- **Error dropped in a builtin argument read from an internal node
  (reported).** For example, `let L = {o: strings.ToUpper((#C & {id:
  "storage"}).out)}` and `r: L.o` give no error where one is expected. Same
  on v0.17.1, rc.1, rc.3 and with the fix. Inside a disjunct within an inline
  struct, it also affects the plain inline-struct case. Found while preparing
  deferred-disjunct-scope; the probe files were in that agent's scratch
  directory.
- **Comprehension adding to a shared selection loses a field (reported,
  old).** `out: x.args & #A`, where a comprehension on `x` adds to `args`,
  loses `b` on v0.17.1 too. This is related to u5 but older. Found while
  preparing share-disjunction-safe.
