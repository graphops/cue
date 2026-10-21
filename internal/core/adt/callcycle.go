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

// callArgsReachCycle reports whether evaluating the arguments of call, a
// cyclic conjunct of n evaluated in env with cycle information ci, may
// re-enter the cycle. Only such a call needs to be postponed until n has
// evidence of a non-cyclic conjunct (see [nodeContext.scheduleConjunct]).
//
// A conjunct is marked cyclic when a reference reaching it closed a cycle
// somewhere above n, and the mark is inherited by every conjunct derived from
// it. References in the derived conjuncts are checked by
// [nodeContext.detectCycle] when they are resolved, so a struct, a value or a
// reference in a cyclic conjunct is evaluated as usual. The arguments of a
// call are evaluated by the call instead, outside that check: a call such as
// json.Validate(x) in a field of x, or matchN(1, [#c]) in a field of #c,
// would expand the structure it validates again at every level, without end.
// The mark on such a call is the only sign of the cycle, and is why the call
// is postponed.
//
// The mark says nothing about the arguments, though. It is also inherited by
// calls whose arguments read only unrelated values. For instance, given
// `b: c, c: b`, if b is looked up while c is evaluated, the reference to c in
// b closes a cycle, and the call in `c: x: f(k)` is cyclic in b.x. Postponing
// such a call makes it a structural cycle unless the node has a value of its
// own, so that the result depends on evaluation order.
//
// An argument can only re-enter the cycle through a reference to a vertex
// that this conjunct reaches as an ancestor: the node itself, one of its
// parents, a vertex whose evaluation is in progress, or a vertex the conjunct
// was already reached through. These are the tests detectCycle applies to the
// vertex a reference resolves to (see [nodeContext.hasAncestor] and
// [CycleInfo.Refs]); here they are applied to the vertex each free reference
// in the arguments resolves to. Any other vertex is evaluated on its own, and
// the references in it are checked by detectCycle in turn. The vertices are
// looked up among the arcs that exist, without evaluating anything;
// references that cannot be resolved that way, and expressions this analysis
// does not model, are assumed to reach the cycle.
func (n *nodeContext) callArgsReachCycle(env *Environment, call *CallExpr, ci CloseInfo) bool {
	for _, a := range call.Args {
		if n.exprReachesCycle(env, ci, a, maxCycleLetDepth) {
			return true
		}
	}
	return false
}

// maxCycleLetDepth bounds the number of let expressions that
// [nodeContext.exprReachesCycle] follows from one let to another.
const maxCycleLetDepth = 4

// exprReachesCycle reports whether a free reference in x, evaluated in env,
// may reach the cycle; see [nodeContext.callArgsReachCycle]. lets is the
// number of let expressions that may still be followed.
func (n *nodeContext) exprReachesCycle(env *Environment, ci CloseInfo, x Expr, lets int) bool {
	return !visitFreeRefs(x, 0, func(path Expr, level int32) bool {
		return !n.pathReachesCycle(env, ci, path, level, lets)
	})
}

// pathReachesCycle reports whether path, a free reference or a selection from
// one, evaluated in env, may reach the cycle. level is the number of
// environments above env in which the reference is resolved, and is -1 for a
// reference to the vertex being evaluated. A nil path stands for an
// expression that is not modeled.
func (n *nodeContext) pathReachesCycle(env *Environment, ci CloseInfo, path Expr, level int32, lets int) bool {
	if l, ok := path.(*LetReference); ok {
		// A let reads what its expression reads.
		e := envUp(env, level)
		return e == nil || lets == 0 || n.exprReachesCycle(e, ci, l.X, lets-1)
	}
	v, external := n.pathVertex(env, path, level, lets)
	switch {
	case external:
		return false // an imported package cannot depend on this one
	case v == nil:
		return true
	case equalDeref(v, n.node) || n.hasAncestor(v):
		return true
	}
	for r := ci.Refs; r != nil; r = r.Next {
		if equalDeref(r.Arc, v) {
			return true
		}
	}
	return false
}

// pathVertex returns the existing vertex that path, a free reference or a
// selection from one, resolves to in env, without evaluating anything; see
// [nodeContext.pathReachesCycle] for level. external reports that path reads
// another package. Otherwise, a nil vertex means that path is not modeled or
// does not resolve to an existing arc.
func (n *nodeContext) pathVertex(env *Environment, path Expr, level int32, lets int) (v *Vertex, external bool) {
	head, labels := splitPath(path)
	switch x := head.(type) {
	case *ImportReference:
		return nil, true

	case *FieldReference:
		if e := envUp(env, level); e != nil {
			v = e.DerefVertex(n.ctx)
		}
		if v != nil {
			v = v.Lookup(x.Label)
		}

	case *ValueReference:
		if level < 0 {
			v = n.node
		} else if e := envUp(env, level); e != nil {
			v = e.DerefVertex(n.ctx)
		}

	case *LetReference:
		// A selection from a let selects from the vertex the expression of
		// the let resolves to, if it is a path itself.
		e := envUp(env, level)
		l, ok := pathLevel(x.X)
		if e == nil || lets == 0 || !ok || l < 0 {
			return nil, false
		}
		if v, external = n.pathVertex(e, x.X, l, lets-1); external {
			return nil, true
		}

	default:
		return nil, false
	}

	for _, l := range labels {
		if v == nil {
			break
		}
		v = v.Lookup(l)
	}
	return v, false
}

// splitPath splits a selection path into the expression it starts from and
// its selectors, in order.
func splitPath(path Expr) (head Expr, labels []Feature) {
	head = path
	for {
		sel, ok := head.(*SelectorExpr)
		if !ok {
			break
		}
		labels = append(labels, sel.Sel)
		head = sel.X
	}
	for i, j := 0, len(labels)-1; i < j; i, j = i+1, j-1 {
		labels[i], labels[j] = labels[j], labels[i]
	}
	return head, labels
}

// pathLevel returns the number of environments above the one path is
// evaluated in that the reference path starts from is resolved in, and -1 for
// a reference to the vertex being evaluated. It reports false if path is not
// a reference to a vertex or a selection from one.
func pathLevel(path Expr) (level int32, ok bool) {
	head, _ := splitPath(path)
	switch x := head.(type) {
	case *FieldReference:
		return x.UpCount, true
	case *LetReference:
		return x.UpCount, true
	case *ValueReference:
		return x.UpCount - 1, true
	}
	return 0, false
}

// envUp returns the environment level steps above env, or nil if there is
// none.
func envUp(env *Environment, level int32) *Environment {
	for range level {
		if env == nil {
			return nil
		}
		env = env.Up
	}
	return env
}

// visitFreeRefs calls yield for each reference to a vertex in x that is not
// bound within x, or for the selection path from it if there is one. When
// depth is 0, yield is passed the number of environments above the one x is
// evaluated in that the reference is resolved in, or -1 for a reference to
// the vertex being evaluated. depth counts the scopes opened so far by the
// expressions enclosing x; they mirror the scopes the compiler counts in
// UpCount. For an expression that is not modeled, yield is called with a nil
// path.
//
// visitFreeRefs stops and returns false as soon as yield returns false.
func visitFreeRefs(x Node, depth int32, yield func(path Expr, level int32) bool) bool {
	all := func(depth int32, xs ...Node) bool {
		for _, x := range xs {
			if x != nil && !visitFreeRefs(x, depth, yield) {
				return false
			}
		}
		return true
	}

	switch x := x.(type) {
	case nil:
		return true

	case *FieldReference, *LetReference, *ValueReference, *ImportReference,
		*SelectorExpr:
		path := x.(Expr)
		head, _ := splitPath(path)
		if _, ok := head.(*ImportReference); ok {
			return yield(path, 0)
		}
		up, ok := pathLevel(head)
		switch {
		case !ok:
			// A selection from an expression that is not a reference.
			return visitFreeRefs(head, depth, yield)
		case up < 0 && depth == 0:
			return yield(path, -1)
		case up < depth:
			return true // bound within x
		}
		return yield(path, up-depth)

	case *LabelReference:
		return true // reads a label, not a vertex

	case *IndexExpr:
		return all(depth, x.X, x.Index)
	case *SliceExpr:
		return all(depth, x.X, x.Lo, x.Hi, x.Stride)
	case *Interpolation:
		for _, p := range x.Parts {
			if !visitFreeRefs(p, depth, yield) {
				return false
			}
		}
		return true
	case *UnaryExpr:
		return visitFreeRefs(x.X, depth, yield)
	case *BinaryExpr:
		return all(depth, x.X, x.Y)
	case *BoundExpr:
		return visitFreeRefs(x.Expr, depth, yield)
	case *OpenExpr:
		return visitFreeRefs(x.X, depth, yield)
	case *CallExpr:
		if !visitFreeRefs(x.Fun, depth, yield) {
			return false
		}
		for _, a := range x.Args {
			if !visitFreeRefs(a, depth, yield) {
				return false
			}
		}
		return true
	case *DisjunctionExpr:
		for _, d := range x.Values {
			if !visitFreeRefs(d.Val, depth, yield) {
				return false
			}
		}
		return true

	case *StructLit:
		for _, d := range x.Decls {
			if !visitFreeRefsDecl(d, depth+1, yield) {
				return false
			}
		}
		return true
	case *ListLit:
		for _, e := range x.Elems {
			if !visitFreeRefsDecl(e, depth+1, yield) {
				return false
			}
		}
		return true

	case *Function:
		// Parameter constraints, defaults and the return type are compiled
		// in the closure scope; only the body opens a scope.
		if !visitFreeRefs(x.Ret, depth, yield) {
			return false
		}
		for _, p := range x.Params {
			if !all(depth, p.Value, p.Default) {
				return false
			}
		}
		return visitFreeRefs(x.Body, depth+1, yield)

	case *Null, *Bool, *Num, *String, *Bytes, *Top, *BasicType, *Bottom,
		*BoundValue, *Builtin, *BuiltinValidator, *ExternalFunc,
		*ExternalValidator, *FuncValue, *Vertex, *Conjunction, *Disjunction,
		*NodeLink:
		// Values, and nodes bound to an absolute environment or vertex.
		return true
	}
	// Dynamic references and expressions that are not modeled.
	return yield(nil, 0)
}

// visitFreeRefsDecl is [visitFreeRefs] for an element of a struct or list
// literal, evaluated at depth.
func visitFreeRefsDecl(d Decl, depth int32, yield func(path Expr, level int32) bool) bool {
	switch x := d.(type) {
	case *Field:
		return visitFreeRefs(x.Value, depth, yield)
	case *LetField:
		return visitFreeRefs(x.Value, depth, yield)
	case *BulkOptionalField:
		return visitFreeRefs(x.Filter, depth, yield) &&
			visitFreeRefs(x.Value, depth, yield)
	case *DynamicField:
		return visitFreeRefs(x.Key, depth, yield) &&
			visitFreeRefs(x.Value, depth, yield)
	case *Ellipsis:
		return visitFreeRefs(x.Value, depth, yield)
	case *Comprehension:
		return visitFreeRefsComprehension(x, depth, yield)
	case Expr:
		return visitFreeRefs(x, depth, yield)
	}
	return yield(nil, 0)
}

// visitFreeRefsComprehension follows the scopes the clauses of a
// comprehension open.
func visitFreeRefsComprehension(x *Comprehension, depth int32, yield func(path Expr, level int32) bool) bool {
	d := depth
	for _, y := range x.Clauses {
		switch c := y.(type) {
		case *ForClause:
			if !visitFreeRefs(c.Src, d, yield) {
				return false
			}
			d++
		case *IfClause:
			if !visitFreeRefs(c.Condition, d, yield) {
				return false
			}
		case *LetClause:
			if !visitFreeRefs(c.Expr, d, yield) {
				return false
			}
			d++
		case *TryClause:
			if c.Expr != nil {
				if !visitFreeRefs(c.Expr, d, yield) {
					return false
				}
				d++
			}
		default:
			return yield(nil, 0)
		}
	}
	if x.Value != nil && !visitFreeRefs(x.Value, d, yield) {
		return false
	}
	if x.Fallback != nil && !visitFreeRefs(x.Fallback, depth, yield) {
		return false
	}
	return true
}
