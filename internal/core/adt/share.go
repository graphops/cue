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

import "iter"

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

// sharesSingleValue reports whether the vertex n shares holds a value that
// is not a disjunction, finalizing the vertex if needed to know its value.
func (n *nodeContext) sharesSingleValue() bool {
	v, ok := n.node.BaseValue.(*Vertex)
	if !ok {
		return false
	}
	// A cyclic reference is left to finalizeSharing, as in that case the
	// vertex may depend on this node. So is a vertex whose evaluation is
	// still in progress: finalizing it now would force its outstanding work
	// from within that evaluation.
	if !v.isFinal() && n.shareCycleType == NoCycle && !finalizeInterrupts(v) {
		v.Finalize(n.ctx)
		v = n.reshareFinalized(v)
	}
	return v.isFinal() && disjunctionOf(v) == nil
}

// reshareFinalized updates the sharing of n after the vertex v it shares
// was finalized and returns the vertex n shares then. If v turned out to
// share a vertex itself, as for a forward reference to a field declared as
// `x: #A`, n shares that vertex instead, as if v had already been finalized
// when its reference was resolved (see processResolver). Otherwise, if n is
// unshared, it would replay the conjuncts of v, whose reference to the
// vertex v now dereferences to is taken for a self-reference, as in `x: x`,
// and dropped.
func (n *nodeContext) reshareFinalized(v *Vertex) *Vertex {
	w := v.DerefNonDisjunct()
	if w == v {
		return v
	}
	if n.node.BaseValue == v {
		n.node.BaseValue = w
		if v.OpenedShared {
			n.node.OpenedShared = true
		}
	}
	for i := range n.shared {
		s := &n.shared[i]
		if s.arc != v {
			continue
		}
		s.arc = w
		if v.OpenedShared {
			s.id.Opened = true
		}
	}
	return w
}

// finalizeInterrupts reports whether finalizing v could interfere with an
// evaluation in progress, which finalizing would force from within.
//
// The evaluator records evaluations in progress on the vertices they apply
// to: a unify call on the stack marks its vertex with the evaluation depth
// (see nodeContext.markDepth), completeNodeTasks marks it as completing, and
// a task being run is in the taskRUNNING state on the node it belongs to and,
// for a comprehension pushed down from a parent, on the arcs it adds fields
// to (scheduler.parentTasks). Finalizing v finalizes the arcs of v and the
// vertices v and its arcs share, recursively, so none of them may be in
// progress. A task running on an ancestor of any of them, such as a
// comprehension or an embedded reference, may still add conjuncts to them.
//
// For example, in
//
//	x: args: {
//		if true {a: "1"}
//		for k, v in out.args {b: v}
//	}
//	out: args: x.args & (#Map | string)
//
// out.args is evaluated by the running comprehension of x.args, which would
// otherwise be frozen before it adds b.
func finalizeInterrupts(v *Vertex) bool {
	var c finalizeCheck
	return c.interrupts(v)
}

// finalizeCheck holds the vertices visited by finalizeInterrupts, as the
// vertices shared by arcs may lead back to vertices visited before.
type finalizeCheck struct {
	seen map[*Vertex]bool
}

// interrupts reports whether v, its ancestors or the vertices finalizing v
// would finalize are in progress; see finalizeInterrupts.
func (c *finalizeCheck) interrupts(v *Vertex) bool {
	for p := v.Parent; p != nil; p = p.Parent {
		if s := p.state; s != nil && p.status != finalized && hasRunningTask(s) {
			return true
		}
	}
	return c.inProgress(v)
}

// inProgress reports whether v, its arcs, or the vertices they share are
// being evaluated.
func (c *finalizeCheck) inProgress(v *Vertex) bool {
	if v.status == finalized {
		return false
	}
	if c.seen[v] {
		return false
	}
	if c.seen == nil {
		c.seen = map[*Vertex]bool{}
	}
	c.seen[v] = true

	if s := v.state; s != nil &&
		(s.evalDepth > 0 || s.isCompleting > 0 || hasRunningTask(s)) {
		return true
	}
	if w, ok := v.BaseValue.(*Vertex); ok && c.interrupts(w) {
		return true
	}
	for _, a := range v.Arcs {
		if c.inProgress(a) {
			return true
		}
	}
	return false
}

// hasRunningTask reports whether a task of n, or one pushed down to n from a
// parent, is running.
func hasRunningTask(n *nodeContext) bool {
	for _, t := range n.tasks {
		if t.state == taskRUNNING {
			return true
		}
	}
	for _, t := range n.parentTasks {
		if t.state == taskRUNNING {
			return true
		}
	}
	return false
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
	n.addShared(c, arc, id)
	n.node.OpenedShared = id.Opened || id.ConjunctOpened

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
