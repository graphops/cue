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

import (
	"math"
	"slices"
)

// noFreeRef is the level reported for an expression that reads no
// environment of its evaluation context.
const noFreeRef = math.MaxInt32

// A clauseBinding is set on each environment that a for, let or binding try
// clause creates. It records what references into the environment read: the
// clause's bound names and, for a for clause, the arc of the iteration. See
// [OpContext.callArgsKey].
type clauseBinding struct {
	clause Node    // *ForClause, *LetClause or *TryClause
	src    *Vertex // the arc a for clause binds; nil otherwise
}

// envReads describes the environments an expression reads, as levels
// relative to the environment in which it is evaluated.
type envReads struct {
	// levels are the distinct levels that free references read, ascending.
	levels []int32
	// unknown is set when the expression contains a node the analysis does
	// not model; it may then read any environment.
	unknown bool
}

// An argsKey identifies the values that the arguments of a call read: one
// interned node per environment level read, chained from the nearest level
// outward, so that equal keys are the same pointer. A nil key means the
// arguments read no environment.
type argsKey struct {
	prev  *argsKey
	level int32
	ident envIdent
	// caller is set only when the reads are unknown: the caller
	// environment, which keys the call exactly.
	caller envKey
}

// envIdent is the identity of what references into one environment read.
type envIdent struct {
	// vertex is the environment's dereferenced vertex or, for a for clause,
	// the dereferenced arc it binds.
	vertex *Vertex
	// label is the environment's DynamicLabel, which label references read.
	label Feature
	// clause is the clause that created a binding environment.
	clause Node
	// bound is, for a let or binding try clause, the key of what its
	// expression reads in the enclosing environment.
	bound *argsKey
}

// callArgsKey returns the memo key for the arguments of call evaluated in
// callEnv.
//
// A call's result depends on its callee, which the memoized entry records
// and matches separately (see [funcCallResult.matches]), and on the values
// its arguments read. Each free reference in the arguments reads one
// environment at a fixed level above callEnv, and what it reads there is
// determined by that environment's identity alone:
//
//   - a struct, list or function activation environment by its vertex (and
//     the label of a pattern instance);
//   - a for clause's iteration environment by the clause and the arc it
//     binds: each evaluation of a comprehension creates new iteration
//     environments, but they bind the same arcs;
//   - a let or binding try clause's environment by the clause and,
//     recursively, what its expression reads.
//
// Keying by these identities, rather than by the caller environment and its
// chain of ancestors, lets calls share a result wherever the values their
// arguments read are the same: in the copies made when a struct holding a
// call is referenced elsewhere (function results and activations are not
// structure shared, see [nodeContext.shareIfPossible]), and in each
// evaluation of a comprehension whose body passes a loop variable to a
// function. Otherwise each such copy or evaluation rebuilds the activation
// and re-finalizes the body, including every call in it.
//
// When the arguments contain a node the analysis does not model, the key is
// the caller environment itself.
func (c *OpContext) callArgsKey(call *CallExpr, callEnv *Environment) *argsKey {
	return c.readsKey(c.callReads(call), callEnv)
}

// readsKey returns the key of the values that an expression with reads r
// reads when it is evaluated in env.
func (c *OpContext) readsKey(r *envReads, env *Environment) *argsKey {
	if env == nil {
		return nil
	}
	if r.unknown {
		return c.internArgsKey(argsKey{caller: env.key(c)})
	}
	var k *argsKey
	e, at := env, int32(0)
	for _, l := range r.levels {
		for ; at < l && e.Up != nil; at++ {
			e = e.Up
		}
		if at < l {
			break // beyond the root environment
		}
		k = c.internArgsKey(argsKey{prev: k, level: l, ident: c.envIdent(e)})
	}
	return k
}

// envIdent returns the identity of what references into e read.
func (c *OpContext) envIdent(e *Environment) envIdent {
	if b := e.binding; b != nil {
		switch x := b.clause.(type) {
		case *ForClause:
			return envIdent{vertex: c.derefRoot(b.src), clause: x}
		case *LetClause:
			return envIdent{clause: x, bound: c.readsKey(c.clauseReads(x, x.Expr), e.Up)}
		case *TryClause:
			return envIdent{clause: x, bound: c.readsKey(c.clauseReads(x, x.Expr), e.Up)}
		}
	}
	return envIdent{vertex: e.DerefVertex(c), label: e.DynamicLabel}
}

// internArgsKey returns the canonical node equal to k.
func (c *OpContext) internArgsKey(k argsKey) *argsKey {
	if p, ok := c.argsKeys[k]; ok {
		return p
	}
	if c.argsKeys == nil {
		c.argsKeys = map[argsKey]*argsKey{}
	}
	p := &k
	c.argsKeys[k] = p
	return p
}

// callReads returns, once per call site, the environments its arguments
// read.
func (c *OpContext) callReads(call *CallExpr) *envReads {
	if r, ok := c.envReads[call]; ok {
		return r
	}
	r := argReads(call)
	c.setEnvReads(call, r)
	return r
}

// clauseReads returns, once per clause, the environments that the
// expression bound by a let or binding try clause reads, relative to the
// environment in which the clause is evaluated.
func (c *OpContext) clauseReads(clause Node, x Expr) *envReads {
	if r, ok := c.envReads[clause]; ok {
		return r
	}
	r := &envReads{}
	freeLevels(x, 0, r)
	r.finish()
	c.setEnvReads(clause, r)
	return r
}

func (c *OpContext) setEnvReads(n Node, r *envReads) {
	if c.envReads == nil {
		c.envReads = map[Node]*envReads{}
	}
	c.envReads[n] = r
}

// argReads computes [OpContext.callReads].
func argReads(call *CallExpr) *envReads {
	r := &envReads{}
	for _, a := range call.Args {
		freeLevels(a, 0, r)
	}
	r.finish()
	return r
}

func (r *envReads) add(l int32) {
	r.levels = append(r.levels, l)
}

func (r *envReads) finish() {
	slices.Sort(r.levels)
	r.levels = slices.Compact(r.levels)
}

// argLevel returns the nearest level that the arguments of call read: 0
// when they contain a node the analysis does not model, and [noFreeRef] when
// they read no environment.
func argLevel(call *CallExpr) int32 {
	r := argReads(call)
	switch {
	case r.unknown:
		return 0
	case len(r.levels) == 0:
		return noFreeRef
	}
	return r.levels[0]
}

// freeLevels adds to r each environment level that a free reference in n
// reads, where level 0 is the environment in which n is evaluated when depth
// is 0, and depth counts the scopes that n's enclosing expression has opened
// so far. The scopes mirror the ones the compiler counts in UpCount: struct
// and list literals, for, let and binding try clauses, and function bodies
// each open one; an else clause is evaluated outside the clauses of its
// comprehension.
//
// A node the analysis does not model sets r.unknown.
func freeLevels(n Node, depth int32, r *envReads) {
	ref := func(up int32) {
		if up >= depth { // otherwise bound within n
			r.add(up - depth)
		}
	}
	all := func(depth int32, xs ...Node) {
		for _, x := range xs {
			if x != nil {
				freeLevels(x, depth, r)
			}
		}
	}

	switch x := n.(type) {
	case nil:

	case *FieldReference:
		ref(x.UpCount)
	case *LabelReference:
		ref(x.UpCount)
	case *LetReference:
		// The let is an arc of the vertex at this level, which determines
		// its value.
		ref(x.UpCount)
	case *DynamicReference:
		ref(x.UpCount)
		freeLevels(x.Label, depth, r)
	case *ValueReference:
		if x.UpCount == 0 {
			// The vertex being evaluated, rather than an environment.
			if depth == 0 {
				r.unknown = true
			}
			return
		}
		ref(x.UpCount - 1)

	case *ImportReference:

	case *SelectorExpr:
		freeLevels(x.X, depth, r)
	case *IndexExpr:
		all(depth, x.X, x.Index)
	case *SliceExpr:
		all(depth, x.X, x.Lo, x.Hi, x.Stride)
	case *Interpolation:
		for _, p := range x.Parts {
			freeLevels(p, depth, r)
		}
	case *UnaryExpr:
		freeLevels(x.X, depth, r)
	case *BinaryExpr:
		all(depth, x.X, x.Y)
	case *BoundExpr:
		freeLevels(x.Expr, depth, r)
	case *OpenExpr:
		freeLevels(x.X, depth, r)
	case *CallExpr:
		freeLevels(x.Fun, depth, r)
		for _, a := range x.Args {
			freeLevels(a, depth, r)
		}
	case *DisjunctionExpr:
		for _, d := range x.Values {
			freeLevels(d.Val, depth, r)
		}

	case *StructLit:
		for _, d := range x.Decls {
			freeLevelsDecl(d, depth+1, r)
		}
	case *ListLit:
		for _, e := range x.Elems {
			freeLevelsDecl(e, depth+1, r)
		}

	case *Function:
		// Parameter constraints, defaults and the return type are compiled
		// in the closure scope; only the body opens a scope.
		all(depth, x.Ret)
		for _, p := range x.Params {
			all(depth, p.Value, p.Default)
		}
		freeLevels(x.Body, depth+1, r)

	case *Null, *Bool, *Num, *String, *Bytes, *Top, *BasicType, *Bottom,
		*BoundValue, *Builtin, *ExternalFunc, *ExternalValidator,
		*BuiltinValidator, *FuncValue, *Vertex, *Conjunction, *Disjunction,
		*NodeLink:
		// Values, and nodes bound to an absolute environment or vertex.

	default:
		r.unknown = true
	}
}

// freeLevelsDecl is [freeLevels] for the elements of a struct or list
// literal, evaluated at depth.
func freeLevelsDecl(d Decl, depth int32, r *envReads) {
	switch x := d.(type) {
	case *Field:
		freeLevelsNode(x.Value, depth, r)
	case *LetField:
		freeLevelsNode(x.Value, depth, r)
	case *BulkOptionalField:
		freeLevelsNode(x.Filter, depth, r)
		freeLevelsNode(x.Value, depth, r)
	case *DynamicField:
		freeLevelsNode(x.Key, depth, r)
		freeLevelsNode(x.Value, depth, r)
	case *Ellipsis:
		freeLevelsNode(x.Value, depth, r)
	case *Comprehension:
		freeLevelsComprehension(x, depth, r)
	case Expr:
		freeLevels(x, depth, r)
	default:
		r.unknown = true
	}
}

// freeLevelsNode is [freeLevels] for an optional expression.
func freeLevelsNode(n Node, depth int32, r *envReads) {
	if n == nil {
		return
	}
	if e, ok := n.(Expr); ok {
		freeLevels(e, depth, r)
		return
	}
	r.unknown = true
}

// freeLevelsComprehension follows the scopes a comprehension's clauses open.
func freeLevelsComprehension(x *Comprehension, depth int32, r *envReads) {
	d := depth
	for _, y := range x.Clauses {
		switch c := y.(type) {
		case *ForClause:
			freeLevelsNode(c.Src, d, r)
			d++
		case *IfClause:
			freeLevelsNode(c.Condition, d, r)
		case *LetClause:
			freeLevelsNode(c.Expr, d, r)
			d++
		case *TryClause:
			if c.Expr != nil {
				freeLevelsNode(c.Expr, d, r)
				d++
			}
		default:
			r.unknown = true
			return
		}
	}
	if x.Value != nil {
		freeLevels(x.Value, d, r)
	}
	if x.Fallback != nil {
		freeLevels(x.Fallback, depth, r)
	}
}
