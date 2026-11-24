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
	// vertex may depend on this node.
	if n.shareCycleType == NoCycle {
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
	delete(n.ctx.argChains, n.node)
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
	n.recordArgChain(id)
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

// A function argument is evaluated under the cycle-reference chain of its
// call, not under that of the callee's body: its conjuncts are marked
// [CycleInfo.IsFuncArg] and carry the chain of the call, which scheduling
// them restores. A vertex of an argument that structure shares another
// vertex, however, holds neither the arcs nor the scheduled conjuncts of
// that vertex. A reference to it, a selection through it or a comprehension
// over it reaches those of the shared vertex instead, which carry no such
// chain, and they are scheduled with the chain of the body. If they make a
// call that this chain already holds, as when the argument is the result of
// an earlier call of the same function,
//
//	F: func(r: _) -> _: {r, x: G(1)}
//	G: func(a: _) -> _: {y: H(a)}
//	H: func(b: _) -> _: b
//	W: func(o: _) -> _: F(o)
//	once:  W({})
//	twice: W(once)
//
// the call is taken for a structural cycle. Therefore, the chain under which
// a vertex of an argument starts sharing is recorded, and what is reached
// through the vertex is scheduled under that chain, as it would be if the
// vertex did not share.

// recordArgChain records the chain of id for the node if id is that of a
// function argument. See [OpContext.argChain].
func (n *nodeContext) recordArgChain(id CloseInfo) {
	if id.IsFuncArg {
		n.ctx.setArgChain(n.node, id.Refs)
	}
}

func (c *OpContext) setArgChain(v *Vertex, refs *RefNode) {
	if c.argChains == nil {
		c.argChains = map[*Vertex]*RefNode{}
	}
	c.argChains[v] = refs
}

// argChain reports the chain of the function argument v is part of, if v,
// or a vertex v indirects to, is recorded as sharing a vertex as part of an
// argument. Of several such vertices the last one, closest to the shared
// vertex, determines the chain. If nonDisjunct is set, argChain stops at a
// disjunct, as [Vertex.DerefNonDisjunct] does.
func (c *OpContext) argChain(v *Vertex, nonDisjunct bool) (refs *RefNode, ok bool) {
	if len(c.argChains) == 0 {
		return nil, false
	}
	for {
		if r, found := c.argChains[v]; found {
			refs, ok = r, true
		}
		arc, isVertex := v.BaseValue.(*Vertex)
		if !isVertex || (nonDisjunct && arc.IsDisjunct) {
			return refs, ok
		}
		v = arc
	}
}

// An argSelection records that the operand of a selection resolved to a
// vertex of a function argument that shares another vertex, with the chain of
// the argument and the vertex in which the selection looks up its field.
type argSelection struct {
	refs *RefNode
	node *Vertex
}

// noteArgSelection records an argSelection in c if ref, the vertex that the
// operand of a selection resolved to, shares a vertex as part of a function
// argument, and node is the vertex in which the field is looked up.
// [processResolver] then schedules a field of node under the chain of the
// argument. An operand that does not share keeps the selection recorded by
// its own operand, if any: its fields lie within the same shared vertex.
func (c *OpContext) noteArgSelection(ref, node *Vertex) {
	if ref == nil {
		return
	}
	if refs, ok := c.argChain(ref, false); ok {
		c.argSelection = &argSelection{refs: refs, node: node}
	}
}

// argRef returns the chain under which n schedules the conjuncts of arc, a
// vertex of a function argument that shares another vertex or a vertex
// reached by a selection through one, which ref resolved to under the chain
// refs: ref followed by the chain of the argument, argRefs. The result keeps
// refs for the order of fields: see [RefNode.Outermost].
func (n *nodeContext) argRef(arc *Vertex, ref Resolver, argRefs, refs *RefNode) *RefNode {
	r := &RefNode{
		Ref:      ref,
		Arc:      deref(arc),
		Node:     deref(n.node),
		Next:     argRefs,
		Depth:    n.depth,
		Len:      1,
		argEntry: true,
		outer:    refs,
	}
	if argRefs != nil {
		r.Len = argRefs.Len + 1
	}
	return r
}

// within reports whether v is the node of s or lies within it.
func (s *argSelection) within(v *Vertex) bool {
	for ; v != nil; v = v.Parent {
		if v == s.node {
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
