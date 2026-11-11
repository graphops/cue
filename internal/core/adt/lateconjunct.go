// Copyright 2026 CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adt

import "slices"

// This file holds the handling of conjuncts that are added to an arc after it
// was finalized.
//
// A finalized arc takes no more conjuncts, so such a late conjunct is dropped.
// That is harmless if the arc already has what the conjunct says, as when a
// comprehension is evaluated again in a disjunct. Otherwise the arc is not the
// unification of its conjuncts, which is an answer that depends on the order of
// evaluation. This happens when a task reads an arc before another task adds to
// it, and the second task only runs after the first:
//
//	n: a: enabled: false
//	r: a: 1
//	for _, _ in r {if n.a == _|_ {r: x: 2}}
//	for _, _ in r {n: a: enabled: true}
//
// The first comprehension reads n.a, which finalizes it. The second iterates
// over r, which the first adds to, so it only runs after it, and yields into
// n.a too late. The comprehensions depend on each other through r and n.a, a
// cycle that is not reported as such. Every reading of the fixed point of the
// program ends in a conflict: there is no value of n.a that the first
// comprehension can have read.
//
// The arc cannot be evaluated again: the tasks that read it have acted on its
// value. So a dropped conjunct that does not agree with the arc is an error,
// whether it conflicts with the arc or narrows it, as a value does for a field
// that is only a type or a default. The error is the one that evaluating the
// conjunct together with those of the arc gives, which is a conflict, a field
// that is not allowed in a closed struct, or that the value changed.
//
// The dropped conjuncts are collected by the node that inserted them, for the
// check once all its tasks are done. This takes the check out of the middle of
// the evaluation of the task, where evaluating the conjunct could finalize
// values that are not ready, and lets conjuncts of an arc be checked together.

// lateConjuncts holds the conjuncts dropped for an arc that was already
// finalized.
type lateConjuncts struct {
	arc *Vertex
	cs  []Conjunct
}

// addLateConjunct records conjunct c, which was dropped because arc, an arc of
// n, was already finalized.
func (n *nodeContext) addLateConjunct(arc *Vertex, c Conjunct) {
	if arc.IsErr() {
		return // the arc reports an error already.
	}
	if p, _ := findConjunct(n.ctx, arc.Conjuncts, c); p >= 0 {
		return // c was inserted before.
	}
	i, ok := n.lateIndex[arc]
	if !ok {
		if n.lateIndex == nil {
			n.lateIndex = make(map[*Vertex]int)
		}
		i = len(n.late)
		n.lateIndex[arc] = i
		n.late = append(n.late, lateConjuncts{arc: arc})
	}
	n.late[i].cs = append(n.late[i].cs, c)
}

// checkLateConjuncts reports an error for each finalized arc of n that was
// given conjuncts which do not agree with it.
func (n *nodeContext) checkLateConjuncts() {
	// Checking may evaluate conjuncts that drop conjuncts in turn. These are
	// those of nodes of their own, not of n, but keep the loop to be safe.
	for len(n.late) > 0 {
		late := n.late
		n.late, n.lateIndex = nil, nil
		for _, l := range late {
			n.checkLateConjunct(l)
		}
	}
}

// checkLateConjunct checks the conjuncts dropped for arc l.arc. The cost is
// that of evaluating each conjunct, as long as it agrees with the arc in an
// evident way, such as by being a value the arc has, as when a comprehension
// yields the same conjunct again. Otherwise the conjuncts are unified with the
// conjuncts of the arc, which costs as much as evaluating the arc again.
func (n *nodeContext) checkLateConjunct(l lateConjuncts) {
	ctx := n.ctx
	arc := l.arc

	// A conjunct that the arc is an instance of adds nothing to it, and does
	// not change the result of unifying it with others.
	cs := slices.DeleteFunc(l.cs, func(c Conjunct) bool {
		if x, ok := c.Elem().(Value); ok && arc.isInstanceOfValue(ctx, x) {
			return true
		}
		w := &Vertex{Label: arc.Label}
		w.AddConjunct(c)
		w.Finalize(ctx)
		return arc.isInstanceOf(ctx, w)
	})
	if len(cs) == 0 {
		return
	}

	// The conjuncts of the arc and the dropped ones are unified in a vertex
	// that is placed as the arc is, rather than starting from the arc itself.
	// The arc, a closed struct if it is that of a definition, would be taken
	// to be an embedding of a definition, and so reject fields that its own
	// conjuncts allow. Evaluating the conjuncts applies the closedness of the
	// arc to the dropped ones as to its own.
	w := arc.reevaluate(ctx, cs)
	b := w.Bottom()
	if b.IsIncomplete() {
		return // the conjuncts may be completed later.
	}
	// The values are compared as evaluation gives them. The arc and w are not
	// evaluated alike: the arguments of a validator in the arc, for instance,
	// were not needed yet, while those in w were, and a function type that
	// the conjunct restates is composed with itself in w.
	if b == nil && Equal(ctx, arc, w, evaluated) {
		return
	}

	// Compare with the result without the dropped conjuncts, so as not to
	// report a difference that has another cause.
	w0 := arc.reevaluate(ctx, nil)
	if w0.Bottom() != nil || b == nil && Equal(ctx, w0, w, evaluated) {
		return
	}

	if b == nil {
		b = n.lateConjunctChanged(arc, w, cs[0])
	} else {
		// The error is not that of a child, which Validate would then find
		// on that child: the finalized arc holds no error.
		e := *b
		e.ChildError = false
		b = &e
	}
	// The arc is the error, as if it had failed while it was evaluated: n
	// reports it as the error of a field, along with those of its other fields.
	b.Node = arc
	arc.AddErr(ctx, b)
}

// lateConjunctChanged returns the error for the arc, which the conjunct c,
// dropped when it was added, changes to w without conflict.
func (n *nodeContext) lateConjunctChanged(arc, w *Vertex, c Conjunct) *Bottom {
	ctx := n.ctx
	defer ctx.PopArc(ctx.PushArc(arc))
	defer ctx.releasePositions(ctx.markPositions())
	ctx.AddPosition(c.Elem())

	var b *Bottom
	if f, ok := arc.addedField(w); ok {
		b = ctx.NewErrf("adding field %v not allowed as field set was already referenced", f.SelectorString(ctx))
	} else {
		b = ctx.NewErrf("circular dependency in evaluation: the value changed after evaluation by a later conjunct")
	}
	return b
}

// addedField returns the label of a field that w, the arc evaluated with other
// conjuncts, has and v does not.
func (v *Vertex) addedField(w *Vertex) (Feature, bool) {
	for _, a := range w.Arcs {
		if a.ArcType > ArcRequired {
			continue
		}
		if x := v.lookupArc(a.Label); x == nil || x.ArcType >= ArcPending {
			return a.Label, true
		}
	}
	return 0, false
}

// reevaluate returns the vertex that is the arc v evaluated again with the
// further conjuncts cs, positioned as v is.
func (v *Vertex) reevaluate(ctx *OpContext, cs []Conjunct) *Vertex {
	w := &Vertex{Parent: v.Parent, Label: v.Label}
	for _, c := range v.Conjuncts {
		w.AddConjunct(c)
	}
	for _, c := range cs {
		w.AddConjunct(c)
	}
	w.Finalize(ctx)
	return w
}

// isInstanceOf reports whether the finalized vertex v is an instance of w, a
// finalized vertex, such that unifying v with w gives v. It is conservative: it
// only holds for values that are clearly equal or more general, and not for
// those that need unification to tell, such as disjunctions, list or struct
// constraints, and closed structs.
//
// The cost is that of w, not of v.
func (v *Vertex) isInstanceOf(ctx *OpContext, w *Vertex) bool {
	v, w = v.DerefValue(), w.DerefValue()

	switch w.BaseValue.(type) {
	case *StructMarker:
		if _, ok := v.BaseValue.(*StructMarker); !ok {
			return false
		}
		if w.ClosedRecursive || w.ClosedNonRecursive || w.PatternConstraints != nil || w.HasEllipsis {
			return false
		}
		for _, a := range w.Arcs {
			if a.Label.IsLet() {
				continue
			}
			b := v.lookupArc(a.Label)
			if a.ArcType != ArcMember || b == nil || b.ArcType != ArcMember || !b.isInstanceOf(ctx, a) {
				return false
			}
		}
		return true

	case Value:
		return len(w.Arcs) == 0 && v.isInstanceOfValue(ctx, w.BaseValue.(Value))
	}
	return false
}

// isInstanceOfValue reports whether the finalized vertex v is an instance of
// x, if x is a value without fields that is evident to be: v is x, a value that
// x is more general than, or any value if x is top.
func (v *Vertex) isInstanceOfValue(ctx *OpContext, x Value) bool {
	switch x := x.(type) {
	case *Top:
		return true

	case *BasicType:
		switch y := v.DerefValue().BaseValue.(type) {
		case *BasicType:
			return y.K&x.K == y.K
		case Value:
			return isConcreteScalar(y) && y.Kind()&x.K == y.Kind()
		}

	case *Num, *String, *Bool, *Bytes, *Null:
		v = v.DerefValue()
		y, ok := v.BaseValue.(Value)
		return ok && len(v.Arcs) == 0 && isConcreteScalar(y) && equalTerminal(ctx, y, x, 0)
	}
	return false
}
