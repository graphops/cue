Branch: cjorge-graphops/cue fix/try-own-arcs-finalize (one commit, a54fe4201), based on master a0768bd98.
Replace #XXXX with the issue number before opening the PR, in the commit message and in the
https://cuelang.org/issue/XXXX link in cue/testdata/comprehensions/try.txtar (and its sync copy).
The PR body is the commit message body.

Title:

internal/core/adt: do not finalize fields a try clause has yet to yield into

Body:

TryClause.yield pre-evaluates a try clause by finalizing an inline
vertex, to find out whether a ?-marked reference fails. That vertex may
reach the fields that the comprehension of the clause yields into. In

    x: a: 5
    try X = x? {x: a: <=2}

it shares x, so finalizing it also finalized x and x.a. Since the
dependency-tracking pushdown of https://cuelang.org/cl/1236923, the
comprehension is a parent task of these arcs, and it had not yielded
yet. When it did, insertConjunct dropped the conjuncts of its body for
the finalized x without an error, so the constraint was lost. A new
field, as in {x: b: 1}, was reported as a cyclic reference to field b
instead.

Mark the task of the comprehension while it pre-evaluates the clause.
While a parent task of a vertex is so marked, Vertex.unify lowers a
request to finalize the vertex to attemptOnly. This covers every path by
which the pre-evaluation reaches these arcs, such as sharing,
finalizeSharing and completeAllArcs. The vertex is finalized later, once
the comprehension has yielded. deferFieldSetKnown no longer exempts the
marked task either: as the current task, it froze the field set of x
during the pre-evaluation, so that a dynamic field, as in {x: ("b"): 1},
was still rejected as added after the field set was referenced.

The exemption is limited to the pre-evaluation, which only decides
whether the clause yields. Other clauses of the same comprehension, such
as an if clause reading a field which its body changes, still finalize
that field, so that the cycle is reported as it is without the try
clause. Exempting every running comprehension instead hides the cycles
of cycle/compbottom2.txtar.

Fixes #XXXX

🤖 Generated with [Claude Code](https://claude.com/claude-code)
