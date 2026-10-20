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

import "math"

// noFreeRef is the level reported for an expression that reads no
// environment of its evaluation context.
const noFreeRef = math.MaxInt32

// callMemoEnv returns the environment that identifies the memo bucket of a
// call evaluated in callEnv: the shallowest ancestor of callEnv that any of
// the call's arguments reads.
//
// A call's result depends on its callee, which the memoized entry records
// and matches separately (see [funcCallResult.matches]), and on its
// arguments. The arguments are expressions resolved in callEnv, and each
// free reference in them reads one fixed ancestor of it. Keying the bucket
// by callEnv itself is correct but too fine: when a struct containing a call
// is referenced elsewhere, its conjuncts are evaluated again in a vertex of
// the referencing location, so each such copy evaluates the call in an
// environment of its own, even though the copy's environments differ from
// the original only below the levels the arguments read. Function results
// and activations are not structure shared (see [nodeContext.shareIfPossible]),
// so this happens for every reference to a function result, and every
// reference repeats the whole call, including the calls in its body.
//
// Keying by the shallowest environment the arguments read lets these copies
// share one result. When an argument reads the innermost environment, the
// bucket is that of callEnv, as before.
func (c *OpContext) callMemoEnv(call *CallExpr, callEnv *Environment) *Environment {
	level := c.callArgLevel(call)
	if level == 0 || callEnv == nil {
		return callEnv
	}
	e := callEnv
	for i := int32(0); i < level && e.Up != nil; i++ {
		e = e.Up
	}
	return e
}

// callArgLevel returns the shallowest level, relative to the environment in
// which call is evaluated, that a free reference in its arguments reads, or
// [noFreeRef] if the arguments read no environment. It is computed once per
// call site.
func (c *OpContext) callArgLevel(call *CallExpr) int32 {
	if l, ok := c.callArgLevels[call]; ok {
		return l
	}
	l := argLevel(call)
	if c.callArgLevels == nil {
		c.callArgLevels = map[*CallExpr]int32{}
	}
	c.callArgLevels[call] = l
	return l
}

// argLevel computes [OpContext.callArgLevel].
func argLevel(call *CallExpr) int32 {
	l := int32(noFreeRef)
	for _, a := range call.Args {
		l = min(l, freeLevel(a, 0))
	}
	return l
}

// freeLevel returns the shallowest environment level that a free reference
// in n reads, where level 0 is the environment in which n is evaluated when
// depth is 0, and depth counts the scopes that n's enclosing expression has
// opened so far. The scopes mirror the ones the compiler counts in UpCount:
// struct and list literals, for, let and binding try clauses, and function
// bodies each open one; an else clause is evaluated outside the clauses of
// its comprehension.
//
// Nodes the analysis does not model report level 0, which keys the memo by
// the evaluation environment itself.
func freeLevel(n Node, depth int32) int32 {
	ref := func(up int32) int32 {
		if up < depth {
			return noFreeRef // bound within n
		}
		return up - depth
	}
	all := func(depth int32, xs ...Node) int32 {
		l := int32(noFreeRef)
		for _, x := range xs {
			if x != nil {
				l = min(l, freeLevel(x, depth))
			}
		}
		return l
	}

	switch x := n.(type) {
	case nil:
		return noFreeRef

	case *FieldReference:
		return ref(x.UpCount)
	case *LabelReference:
		return ref(x.UpCount)
	case *LetReference:
		// The let expression is evaluated in the let's own scope, which is
		// this level or a shallower one.
		return ref(x.UpCount)
	case *DynamicReference:
		return min(ref(x.UpCount), freeLevel(x.Label, depth))
	case *ValueReference:
		if x.UpCount == 0 {
			// The vertex being evaluated.
			if depth == 0 {
				return 0
			}
			return noFreeRef
		}
		return ref(x.UpCount - 1)

	case *ImportReference:
		return noFreeRef

	case *SelectorExpr:
		return freeLevel(x.X, depth)
	case *IndexExpr:
		return all(depth, x.X, x.Index)
	case *SliceExpr:
		return all(depth, x.X, x.Lo, x.Hi, x.Stride)
	case *Interpolation:
		l := int32(noFreeRef)
		for _, p := range x.Parts {
			l = min(l, freeLevel(p, depth))
		}
		return l
	case *UnaryExpr:
		return freeLevel(x.X, depth)
	case *BinaryExpr:
		return all(depth, x.X, x.Y)
	case *BoundExpr:
		return freeLevel(x.Expr, depth)
	case *OpenExpr:
		return freeLevel(x.X, depth)
	case *CallExpr:
		l := freeLevel(x.Fun, depth)
		for _, a := range x.Args {
			l = min(l, freeLevel(a, depth))
		}
		return l
	case *DisjunctionExpr:
		l := int32(noFreeRef)
		for _, d := range x.Values {
			l = min(l, freeLevel(d.Val, depth))
		}
		return l

	case *StructLit:
		l := int32(noFreeRef)
		for _, d := range x.Decls {
			l = min(l, freeLevelDecl(d, depth+1))
		}
		return l
	case *ListLit:
		l := int32(noFreeRef)
		for _, e := range x.Elems {
			l = min(l, freeLevelDecl(e, depth+1))
		}
		return l

	case *Function:
		// Parameter constraints, defaults and the return type are compiled
		// in the closure scope; only the body opens a scope.
		l := all(depth, x.Ret)
		for _, p := range x.Params {
			l = min(l, all(depth, p.Value, p.Default))
		}
		return min(l, freeLevel(x.Body, depth+1))

	case *Null, *Bool, *Num, *String, *Bytes, *Top, *BasicType, *Bottom,
		*BoundValue, *Builtin, *ExternalFunc, *ExternalValidator,
		*BuiltinValidator, *FuncValue, *Vertex, *Conjunction, *Disjunction,
		*NodeLink:
		// Values, and nodes bound to an absolute environment or vertex.
		return noFreeRef
	}
	return 0
}

// freeLevelDecl is [freeLevel] for the elements of a struct or list literal,
// evaluated at depth.
func freeLevelDecl(d Decl, depth int32) int32 {
	min2 := func(a, b Node) int32 {
		return min(freeLevelNode(a, depth), freeLevelNode(b, depth))
	}
	switch x := d.(type) {
	case *Field:
		return freeLevelNode(x.Value, depth)
	case *LetField:
		return freeLevelNode(x.Value, depth)
	case *BulkOptionalField:
		return min2(x.Filter, x.Value)
	case *DynamicField:
		return min2(x.Key, x.Value)
	case *Ellipsis:
		return freeLevelNode(x.Value, depth)
	case *Comprehension:
		return freeLevelComprehension(x, depth)
	case Expr:
		return freeLevel(x, depth)
	}
	return 0
}

// freeLevelNode is [freeLevel] for an optional expression.
func freeLevelNode(n Node, depth int32) int32 {
	if n == nil {
		return noFreeRef
	}
	if e, ok := n.(Expr); ok {
		return freeLevel(e, depth)
	}
	return 0
}

// freeLevelComprehension follows the scopes a comprehension's clauses open.
func freeLevelComprehension(x *Comprehension, depth int32) int32 {
	l := int32(noFreeRef)
	d := depth
	for _, y := range x.Clauses {
		switch c := y.(type) {
		case *ForClause:
			l = min(l, freeLevelNode(c.Src, d))
			d++
		case *IfClause:
			l = min(l, freeLevelNode(c.Condition, d))
		case *LetClause:
			l = min(l, freeLevelNode(c.Expr, d))
			d++
		case *TryClause:
			if c.Expr != nil {
				l = min(l, freeLevelNode(c.Expr, d))
				d++
			}
		default:
			return 0
		}
	}
	if x.Value != nil {
		l = min(l, freeLevel(x.Value, d))
	}
	if x.Fallback != nil {
		l = min(l, freeLevel(x.Fallback, depth))
	}
	return l
}
