// Copyright 2024 CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adt

import (
	"iter"
	"slices"
)

// This file contains logic regarding structure sharing.

// Notes
//
// TODO:
// We may want to consider tracking closedness in parallel to the Vertex
// structure, for instance in a CloseInfo or in a cue.Value itself.
//
//     reg: {}
//     #def: sub: reg
//
// By tracking closedness inside the CloseInfo, we can still share the
// structure and only have to change
//
// Maybe this is okay, though, as #Def itself can be shared, at least.

// sharedConjunct is a conjunct a shared node holds without having scheduled
// it: the reference that started the sharing, one resolving to the same
// vertex, or a definition that vertex is already an instance of. Each is
// scheduled in turn if the node is unshared.
type sharedConjunct struct {
	c   Conjunct
	arc *Vertex
	id  CloseInfo
}

func (n *nodeContext) unshare() {
	n.noSharing = true

	if !n.isShared {
		return
	}
	n.dropSharing()
	for _, s := range n.shared {
		n.scheduleVertexConjuncts(s.c, s.arc, s.id)
	}
}

// plainApplication reports whether conjunct c, with close info id, applies
// its value directly: not embedded and not opened. Only then does applying a
// definition close the node exactly as the definition does, which the
// redundancy of a repeated definition conjunct relies on.
func plainApplication(c Conjunct, id CloseInfo) bool {
	return !c.CloseInfo.Opened && !c.CloseInfo.ConjunctOpened &&
		!id.Opened && !id.ConjunctOpened && notEmbedded(id)
}

// notEmbedded reports whether a conjunct with close info id is applied to
// the node directly rather than through an embedding. A definition the
// shared value applied plainly (closed) makes such a conjunct redundant
// whether it is plain or opened (D...): an opened definition adds the same
// constraints and no closedness.
func notEmbedded(id CloseInfo) bool {
	return !id.FromEmbed && id.enclosingEmbed == 0
}

// appliedDef is a definition whose conjuncts were unified directly into a
// vertex, and whether that application closed the vertex (a plain reference)
// or only constrained it (an opened one, D...).
type appliedDef struct {
	def    *Vertex
	closed bool
}

// recordAppliedDef records that the conjuncts of definition arc are unified
// directly into v, closing it or not.
//
// The list is never modified in place, as a clone of v made for a disjunct
// starts out sharing it.
func (v *Vertex) recordAppliedDef(arc *Vertex, closed bool) {
	for i, a := range v.appliedDefs {
		if a.def == arc {
			if closed && !a.closed {
				v.appliedDefs = slices.Clone(v.appliedDefs)
				v.appliedDefs[i].closed = true
			}
			return
		}
	}
	v.appliedDefs = append(slices.Clip(v.appliedDefs), appliedDef{arc, closed})
}

// appliesDef reports whether v, or a value it shares, already unified the
// conjuncts of definition def, closing it if needClosed, so that unifying def
// again in that mode adds nothing.
func (v *Vertex) appliesDef(def *Vertex, needClosed bool) bool {
	for {
		if v == def {
			return true
		}
		for _, a := range v.appliedDefs {
			if a.def == def && (a.closed || !needClosed) {
				return true
			}
		}
		w, ok := v.BaseValue.(*Vertex)
		if !ok {
			return false
		}
		v = w
	}
}

// skipRedundantDef handles a definition conjunct arriving at a node that
// structure shares a value which already applied it: the conjunct is held
// as shared instead of scheduled, so the node keeps sharing, and it is
// scheduled as usual if the node is unshared. If the shared value is itself
// the definition and arc is a value that applied it, the node shares arc
// instead. It reports whether arc was handled.
//
// This extends [nodeContext.sharedClosedBy], which finds the definition
// among the conjuncts of the shared value, to definitions that value
// applied in any other way, for instance through a field of an opened
// definition, to a definition arriving before the value, and to opened
// definitions.
//
// A plain definition closes a node that shares an opened reference
// (x... & #D), so it is not redundant then, and the node stops sharing as
// usual.
//
// A node within a function argument is left as before. A reference that
// looks up a field of a parameter sharing its argument reaches the field of
// the shared value, whose conjuncts are then scheduled with the
// cycle-reference chain of the callee's body, not the one pinned by the
// argument (see [CycleInfo.IsFuncArg]). If such a conjunct calls the
// function in progress, which is nesting, the call is taken for recursion
// and reported as a structural cycle. Not skipping the definition unshares
// the node, so the field is evaluated within the argument, under the
// caller's chain, as for a node that shares nothing.
func (n *nodeContext) skipRedundantDef(c Conjunct, arc *Vertex, id CloseInfo) bool {
	if !n.isShared || !notEmbedded(id) ||
		inFuncArg(n.node) {
		return false
	}
	cur, ok := n.node.BaseValue.(*Vertex)
	if !ok {
		return false
	}
	closes := plainApplication(c, id)
	switch {
	case arc.Label.IsDef() && !(closes && n.node.OpenedShared) &&
		cur.appliesDef(arc, closes):
		n.addShared(c, arc, id)
		return true

	case cur.Label.IsDef() && len(n.shared) == 1 && notEmbedded(n.shared[0].id):
		prev := n.shared[0]
		prevCloses := plainApplication(prev.c, prev.id)
		if (prevCloses && !closes) || !arc.appliesDef(cur, prevCloses) {
			return false
		}
		// Share arc, which includes the definition, rather than the
		// definition itself.
		n.isShared = false
		n.node.IsShared = false
		n.node.OpenedShared = false
		n.node.BaseValue = n.origBaseValue
		n.shared = n.shared[:0]
		if !n.shareIfPossible(c, arc, id) {
			n.scheduleVertexConjuncts(prev.c, prev.arc, prev.id)
			return false
		}
		arc.getState(n.ctx)
		n.addShared(prev.c, prev.arc, prev.id)
		return true
	}
	return false
}

// inFuncArg reports whether v is a parameter of a function call, or a node
// within one, whose argument conjunct pins the caller's cycle-reference chain.
// The fields of an argument that is a struct are scheduled without the mark
// that the argument itself carries, so it is found on the parameter.
func inFuncArg(v *Vertex) bool {
	for ; v != nil; v = v.Parent {
		if v.IsDynamic && len(v.Conjuncts) > 0 && v.Conjuncts[0].CloseInfo.IsFuncArg {
			return true
		}
	}
	return false
}

// sharesSingleValue reports whether the vertex n shares holds a value that
// is not a disjunction. If n is being finalized, the vertex is finalized
// first if needed to know its value.
func (n *nodeContext) sharesSingleValue() bool {
	v, ok := n.node.BaseValue.(*Vertex)
	if !ok {
		return false
	}
	// The vertex is finalized only while this node is, as finalizeSharing
	// does once the node is complete. A node evaluated in part, such as for
	// a selection, may be evaluated from within an evaluation the vertex
	// depends on, such as a comprehension of the vertex, which finalizing
	// the vertex would force before the comprehension adds its fields.
	// A cyclic reference is left to finalizeSharing, as in that case the
	// vertex may depend on this node.
	if n.isFinalizing && n.shareCycleType == NoCycle {
		v.Finalize(n.ctx)
	}
	return v.isFinal() && disjunctionOf(v) == nil
}

// hasResolver reports whether a disjunct of x is a reference, possibly in a
// conjunction, which is the only kind of disjunct that can keep sharing.
func hasResolver(x *DisjunctionExpr) bool {
	for _, d := range x.Values {
		for y := range andOperands(d.Val) {
			if _, ok := y.(Resolver); ok {
				return true
			}
		}
	}
	return false
}

// andOperands yields the operands of a conjunction x, descending into nested
// conjunctions, x itself for any other expression, and nothing for a nil x.
func andOperands(x Expr) iter.Seq[Expr] {
	return func(yield func(Expr) bool) {
		walkAndOperands(x, yield)
	}
}

func walkAndOperands(x Expr, yield func(Expr) bool) bool {
	if b, ok := x.(*BinaryExpr); ok && b.Op == AndOp {
		return walkAndOperands(b.X, yield) && walkAndOperands(b.Y, yield)
	}
	return x == nil || yield(x)
}

// dropSharing ends sharing without replaying the shared vertex, for a
// node whose value is computed elsewhere, such as in its disjuncts.
func (n *nodeContext) dropSharing() {
	if !n.isShared {
		return
	}
	n.isShared = false
	n.noSharing = true
	n.node.IsShared = false
	n.node.OpenedShared = false
	n.node.ClosedShared = false
	n.decShared()

	// TODO: the use of cycle for BaseValue is getting increasingly outdated.
	// Find another mechanism once we get rid of the old evaluator.
	n.node.BaseValue = n.origBaseValue
}

// finalizeSharing should be called when it is known for sure a node can be
// shared.
func (n *nodeContext) finalizeSharing() {
	n.decShared()
	if !n.isShared {
		return
	}
	switch v := n.node.BaseValue.(type) {
	case *Vertex:
		if n.shareCycleType == NoCycle {
			v.Finalize(n.ctx)
			// See the TODO in unify.go for toFinalize.
			// } else if !v.isFinal() {
			// 	// TODO: ideally we just handle cycles in optional chains directly,
			// 	// rather than relying on this mechanism. This requires us to add
			// 	// a mechanism to detect that.
			// 	n.ctx.toFinalize = append(n.ctx.toFinalize, v)
		}
		// If state.parent is non-nil, we determined earlier that this Vertex
		// is not rooted and that it can safely be shared. Because it is
		// not-rooted, though, it will not have a path location, resulting in
		// bad error messages, and in some cases dropped errors. To avoid this,
		// we reset the parent and label of the Vertex so that its path reflects
		// its assigned location.
		if v.state != nil && v.state.parent != nil {
			v.Parent = v.state.parent
		}
	case *Bottom:
		// An error trumps sharing. We can leave it as is.
	default:
		panic("unreachable")
	}
}

func (n *nodeContext) addShared(c Conjunct, arc *Vertex, id CloseInfo) {
	if len(n.shared) == 0 || n.shareCycleType < id.CycleType {
		n.shareCycleType = id.CycleType
	}
	// A reference that does not open the vertex wins, in either order. One
	// naming a definition, or sharing one as arc, the vertex finally shared,
	// stops [Vertex.sharesOpened] at this node.
	if !id.Opened && !id.ConjunctOpened {
		n.node.OpenedShared = false
		if namesDefinition(c.Expr(), id) || arc.Label.IsDef() {
			n.node.ClosedShared = true
		}
	}

	// At this point, the node may still be unshared at a later point. For this
	// purpose we need to keep the retain count above zero until all conjuncts
	// have been processed and it is clear that sharing is possible. Delaying
	// such a count should not hurt performance, as a shared node is completed
	// anyway.
	n.shared = append(n.shared, sharedConjunct{c, arc, id})
}

func (n *nodeContext) decShared() {
	if n.shareDecremented {
		return
	}
	n.shareDecremented = true
	for _, s := range n.shared {
		n.updateConjunctInfo(n.node.Kind(), s.id, 0)
	}
}

func (n *nodeContext) share(c Conjunct, arc *Vertex, id CloseInfo) {
	if n.isShared {
		panic("already sharing")
	}
	n.origBaseValue = n.node.BaseValue
	n.node.BaseValue = arc
	n.node.IsShared = true
	n.isShared = true
	n.node.OpenedShared = id.Opened || id.ConjunctOpened
	n.addShared(c, arc, id)

	if arc.IsDetached() && arc.MayAttach() { // TODO: Second check necessary?
		// This node can safely be shared. Since it is not rooted, though, it
		// does not have a path location. Instead of setting the parent path
		// directly, though, we record the prospective parent in the state: as
		// the evaluator uses the Parent field during evaluation, setting the
		// field directly here can result in incorrect evaluation results.
		// Setting the parent in the state instead allows us to defer setting
		// Parent until it is safe to do so..
		if s := arc.getState(n.ctx); s != nil {
			s.parent = n.node
		}
	}
}

func (n *nodeContext) shareIfPossible(c Conjunct, arc *Vertex, id CloseInfo) bool {
	if !n.ctx.Sharing {
		return false
	}

	// We disallow sharing for any Arcs, even pending ones, to be defensive.
	// CUE currently does not always unwind sharing properly in the precense of
	// pending arcs. See, for instance:
	//
	// 		a: X
	// 		if true {
	// 			a: b: c: e: 1 // ensure 'e' is added
	// 		}
	// 		X: b: Y
	// 		Y: c: d: int
	//
	// TODO: allow sharing in the precense of pending or not present arcs.
	if len(n.node.Arcs) > 0 {
		return false
	}

	// See Issue #3801: structure sharing seems to be broken for non-rooted
	// values. We disable sharing for now.
	// TODO: make sharing work again for non-rooted structs.
	if arc.nonRooted || arc.IsDynamic {
		return false
	}

	// We do not allowing sharing if the conjunct has a cycle. Sharing is only
	// possible if there is a single conjunct. We want to further evaluate this
	// conjunct to force recognition of a structural cycle.
	if id.CycleType == IsCyclic && (n.node.nonRooted || n.node.IsDynamic) {
		return false
	}

	if n.noSharing || n.isShared || n.ctx.errs != nil {
		return false
	}

	// The inline vertex pre-evaluating a try clause body does not share a
	// vertex the clause has yet to yield into, as in
	//
	//     x: r: "4k" | "32k"
	//     try _ = x.r? {x: r: "zz"}
	//
	// Evaluating the shared vertex resolves its disjunction, and a conjunct
	// arriving at a node that has done so is lost. The inline vertex
	// evaluates the conjuncts of the vertex instead, leaving it untouched.
	if n.inTryBody() && arc.state != nil && arc.state.hasPreEvaluatingParentTask() {
		return false
	}

	// An open value is not shared into a recursively closed node, as in
	//
	//     reg: {}
	//     #def: sub: reg
	//
	// since the node would then take its closedness from the open value.
	//
	// TODO: come up with a mechanism to allow this case.
	if n.node.ClosedRecursive && !arc.ClosedRecursive && !arc.closedByConjunct() {
		return false
	}

	// Sharing let expressions is not supported and will result in unmarked
	// structural cycles. Processing will still terminate, but printing the
	// result will result in an infinite loop.
	//
	// TODO: allow this case.
	if arc.Label.IsLet() {
		return false
	}

	n.share(c, arc, id)
	return true
}

// Vertex values that are held in BaseValue will be wrapped in the following
// order:
//
//    disjuncts -> (shared | computed | data)
//
// DerefDisjunct
//   - get the current value under computation
//
// DerefValue
//   - get the value the node ultimately represents.
//
// derefSharedDisjunct
//   - like DerefValue, but stop at a disjunct that shares a vertex.
//

// DerefValue unrolls indirections of Vertex values. These may be introduced,
// for instance, by temporary bindings such as comprehension values.
// It returns v itself if v does not point to another Vertex.
func (v *Vertex) DerefValue() *Vertex {
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok {
			return v
		}
		v = arc
	}
}

// derefSharedDisjunct is like [Vertex.DerefValue], but stops at a disjunct
// that structure shares a vertex, as the node holding the disjunct does not
// share it.
func (v *Vertex) derefSharedDisjunct() *Vertex {
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok {
			return v
		}
		v = arc
		if v.IsDisjunct && v.IsShared {
			return v
		}
	}
}

// DerefDisjunct indirects a node that points to a disjunction.
func (v *Vertex) DerefDisjunct() *Vertex {
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok || !arc.IsDisjunct {
			return v
		}
		v = arc
	}
}

// DerefNonDisjunct indirects a node that points to a disjunction.
func (v *Vertex) DerefNonDisjunct() *Vertex {
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok || arc.IsDisjunct {
			return v
		}
		v = arc
	}
}

// sharesOpened reports whether v, or a node v indirects to as in
// [Vertex.DerefValue], shares a vertex through an opened reference,
// stopping at a node with ClosedShared set. Sharing does not copy
// OpenedShared, as the shared vertex may not be evaluated yet.
func (v *Vertex) sharesOpened() bool {
	for !v.ClosedShared {
		if v.OpenedShared {
			return true
		}
		arc, ok := v.BaseValue.(*Vertex)
		if !ok {
			return false
		}
		v = arc
	}
	return false
}

// derefResolved indirects the vertex a reference resolved to, like
// DerefNonDisjunct, but not past a function parameter that shares its
// argument.
//
// The argument conjunct of a parameter carries the cycle-reference chain of
// the call (see [CycleInfo.IsFuncArg]), and scheduling it replaces the chain
// of the callee's body that refers to the parameter. Dereferencing the
// parameter to the shared argument value would instead schedule the
// conjuncts of that value with the body's chain. If the value is the result
// of an earlier call of the same call site, as in
//
//	W: func(o: _) -> _: F(o.d)
//	once:  W({d: {}})
//	twice: W({d: once})
//
// its conjuncts reach that call site again, which is then taken for a
// structural cycle. Stopping at the parameter schedules its own conjuncts,
// as if it were not shared: the argument is resolved again under the
// caller's chain, and its value may still be shared from there.
func (v *Vertex) derefResolved() *Vertex {
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok || arc.IsDisjunct || v.sharesFuncArg() {
			return v
		}
		v = arc
	}
}

// sharesFuncArg reports whether v is a function parameter that structure
// shares its argument. Parameters are dynamic arcs of a call's activation
// holding the argument conjunct.
func (v *Vertex) sharesFuncArg() bool {
	if !v.IsShared || !v.IsDynamic {
		return false
	}
	for c := range v.LeafConjuncts() {
		if c.CloseInfo.IsFuncArg {
			return true
		}
	}
	return false
}

// DerefNonRooted indirects a node that points to a value that is not rooted.
// This includes structure-shared nodes that point to a let field: let fields
// may or may not be part of a struct, and thus should be treated as non-rooted.
func (v *Vertex) DerefNonRooted() *Vertex {
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok || arc.IsDisjunct || (v.IsShared && !arc.Label.IsLet()) {
			return v
		}
		v = arc
	}
}

// DerefNonShared finds the indirection of an arc that is not the result of
// structure sharing. This is especially relevant when indirecting disjunction
// values.
func (v *Vertex) DerefNonShared() *Vertex {
	if v.state != nil && v.state.isShared {
		return v
	}
	for {
		arc, ok := v.BaseValue.(*Vertex)
		if !ok {
			return v
		}
		v = arc
	}
}

// closedByConjunct reports whether v, whose conjuncts have not been scheduled
// yet, holds a conjunct from a definition. Scheduling that conjunct closes v,
// so until then the flag is stale, which is how a forward reference finds it.
func (v *Vertex) closedByConjunct() bool {
	if v.isInitialized() {
		return false
	}
	for c := range v.LeafConjuncts() {
		if c.CloseInfo.FromDef {
			return true
		}
		// A direct definition reference closes v once it is resolved.
		for x := range andOperands(directExpr(c)) {
			if isDef, _ := IsDef(x); isDef {
				return true
			}
		}
	}
	return false
}

// directExpr returns the expression of a conjunct that applies to its vertex
// directly, neither opened nor embedded, and nil for any other conjunct. A
// comprehension is not an expression, as it may not have yielded its value.
func directExpr(c Conjunct) Expr {
	ci := c.CloseInfo
	if ci.Opened || ci.ConjunctOpened || ci.FromEmbed {
		return nil
	}
	x, _ := c.Elem().(Expr)
	return x
}

// sharedClosedBy reports whether the vertex n shares holds a direct conjunct
// that refers to the definition def, so that it already is an instance of it.
func (n *nodeContext) sharedClosedBy(def *Vertex) bool {
	if !def.Label.IsDef() {
		return false
	}
	shared, ok := n.node.BaseValue.(*Vertex)
	if !ok {
		return false
	}
	for c := range shared.LeafConjuncts() {
		if c.Env == nil {
			continue
		}
		for x := range andOperands(directExpr(c)) {
			if isDef, _ := IsDef(x); !isDef {
				continue
			}
			if v, _ := n.ctx.Lookup(c.Env, x.(Resolver)); v != nil && equalDeref(v, def) {
				return true
			}
		}
	}
	return false
}
