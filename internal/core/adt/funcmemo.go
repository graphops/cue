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

// A bindKind tells what a vertex that a comprehension clause creates to bind
// its variables binds. See [Vertex.binds].
type bindKind uint8

const (
	bindsNothing bindKind = iota

	// bindsIteration marks the vertex of one iteration of a for clause. Its
	// value arc, if any, has the iterated arc as its BaseValue, and its key
	// arc holds that arc's label.
	bindsIteration

	// bindsValue marks the vertex of a let clause or a binding try clause.
	// It has a single arc, whose single conjunct is the bound expression in
	// the environment enclosing the clause.
	bindsValue
)

// envReads describes the environments an expression reads, as levels
// relative to the environment in which it is evaluated.
type envReads struct {
	// levels are the distinct levels that free references read, ascending.
	levels []int32
	// exact are the levels from which a let field is read. A let field may
	// have a value per environment (see [Vertex.MultiLet]), so such a level
	// is keyed by the environment itself.
	exact []int32
	// unknown is set when the expression contains a node that [walkEscapes]
	// does not model; it may then read any environment.
	unknown bool
}

// An argsKey identifies the values that an expression reads from one
// environment level, chained through prev to the levels further out, which
// are interned so that equal chains are the same pointer. The zero value
// means the expression reads no environment.
type argsKey struct {
	prev *argsKey
	envIdent
}

// A callKey identifies the values that the arguments of a call read. The two
// nearest levels are held by value, as they are typically a function
// activation or a comprehension iteration created for this evaluation
// alone, so that computing the key allocates nothing for the arguments of
// most calls; next.prev chains the levels further out.
type callKey struct {
	near, next argsKey
}

// envIdent is the identity of what references into one environment read.
//
// Its fields are pointers and 32-bit integers without padding, so that a
// key holding it is hashed and compared as plain memory.
type envIdent struct {
	// vertex is the environment's dereferenced vertex or, for the iteration
	// of a for clause, the dereferenced arc it binds.
	vertex *Vertex
	// up is the environment's parent, set when a let field is read from it
	// or when what the arguments read is not known: with vertex and label
	// it is the environment's [funcEnvKey], which identifies the
	// environment itself.
	up *Environment
	// expr is, for a let or binding try clause, what the bound expression
	// reads, which identifies the expression (see [OpContext.exprReads]),
	// and bound the key of what it reads in the enclosing environment.
	expr  *envReads
	bound *argsKey
	// label is the environment's DynamicLabel, which label references read.
	label Feature
	// level is the environment's level above the one in which the
	// expression is evaluated, or [unknownReads].
	level int32
}

// A callerContext holds the parts of the caller's CloseInfo that the
// arguments of a call are evaluated with and that can make two callers
// compute different activations from the same values. The argument
// conjuncts of an activation carry the caller's CloseInfo (see
// [FuncValue.call]): whether the call is in a definition decides whether the
// argument arcs are closed, and so whether a body that embeds an argument
// closes the struct it builds; whether it is in an embedding or opened
// decides how definitions referred to in the arguments are tracked for
// closedness (see [nodeContext.addResolver]); the priority decides between
// defaults of different layers; and a cyclic mark postpones the arguments.
//
// The rest of the CloseInfo is not keyed. Its IDs name the struct literals
// of the caller and are new in each evaluation; an argument arc starts
// without closedness requirements of its own, and depends on the IDs only
// through the flags above. The cycle references that the arguments keep
// (see [CycleInfo.IsFuncArg]) only decide whether following a reference in
// an argument is cut as a structural cycle. An argument that is cut for
// one caller reads the same vertices for any other, and so reaches the same
// cycle there, one copy further down.
type callerContext struct {
	flags    uint32 // the caller* bits below, and the cycle type above them
	priority int32
}

const (
	callerFromDef uint32 = 1 << iota
	callerFromEmbed
	callerTopDef
	callerOpened
	callerConjunctOpened
	callerInEmbedding // enclosingEmbed is set
	callerNoOuter     // outerID is zero

	callerCycleTypeShift = 8
)

func makeCallerContext(ci CloseInfo) callerContext {
	f := uint32(ci.CycleType) << callerCycleTypeShift
	for _, x := range [...]struct {
		set bool
		bit uint32
	}{
		{ci.FromDef, callerFromDef},
		{ci.FromEmbed, callerFromEmbed},
		{ci.TopDef, callerTopDef},
		{ci.Opened, callerOpened},
		{ci.ConjunctOpened, callerConjunctOpened},
		{ci.enclosingEmbed != 0, callerInEmbedding},
		{ci.outerID == 0, callerNoOuter},
	} {
		if x.set {
			f |= x.bit
		}
	}
	return callerContext{flags: f, priority: int32(ci.Priority)}
}

// unknownReads is the level of the key of arguments whose reads are not
// known; the key then holds the caller environment.
const unknownReads = -1

// callArgsKey returns the memo key for the arguments of call evaluated in
// callEnv.
//
// A call computes its result from its callee, which the memo key holds
// separately (see [funcCallResultKey]), and from the values its arguments
// read. References are lexical, so each free reference in the arguments
// reads one environment at a fixed level above callEnv, created by the same
// scope in every evaluation of the call. What it reads there is determined
// by:
//
//   - for a struct, list or function activation environment, its vertex and,
//     for a field instantiated from a pattern, its label;
//   - for the iteration of a for clause, the arc it binds: each evaluation
//     of a comprehension creates new iteration environments, but they bind
//     the same arcs;
//   - for a let or binding try clause, the bound expression and,
//     recursively, what it reads in the enclosing environment;
//   - for an environment from which a let field is read, the environment
//     itself, as a let field a vertex receives from several environments
//     has a value per environment (see [LetReference.resolve]).
//
// Keying by these identities, rather than by the caller environment, lets
// calls share a result wherever the values their arguments read are the
// same: in the copies made when a struct holding a call is referenced
// elsewhere (function results and activations are not structure shared,
// see [nodeContext.shareIfPossible]), and in each evaluation of a
// comprehension whose body passes a loop variable to a function.
//
// A hit reuses only the activation: each use of the result schedules the
// body, the return type and the constraints of parameters again under its
// own CloseInfo. What the activation keeps of the caller's CloseInfo is
// keyed separately; see [callerContext].
//
// When the arguments contain a node that walkEscapes does not model, or
// read beyond the root environment, the key is the caller environment.
func (c *OpContext) callArgsKey(call *CallExpr, callEnv *Environment) callKey {
	r, ok := c.callReads[call]
	if !ok {
		r = argReads(call)
		if c.callReads == nil {
			c.callReads = map[*CallExpr]*envReads{}
		}
		c.callReads[call] = r
	}
	return c.readsKey(r, callEnv)
}

// readsKey returns the key of the values that an expression with reads r
// reads when it is evaluated in env.
func (c *OpContext) readsKey(r *envReads, env *Environment) callKey {
	if env == nil || len(r.levels) == 0 && !r.unknown {
		return callKey{}
	}
	if r.unknown {
		return callKey{near: c.callerKey(env)}
	}
	var buf [4]*Environment
	envs := buf[:0]
	e, at := env, int32(0)
	for _, l := range r.levels {
		for ; at < l && e.Up != nil; at++ {
			e = e.Up
		}
		if at < l {
			// Beyond the root environment: not modeled.
			return callKey{near: c.callerKey(env)}
		}
		envs = append(envs, e)
	}
	var k callKey
	var prev *argsKey
	for i := len(envs) - 1; i >= 0; i-- {
		l := r.levels[i]
		a := argsKey{prev: prev, envIdent: c.envIdent(envs[i])}
		a.level = l
		if len(r.exact) > 0 && slices.Contains(r.exact, l) {
			a.up = envs[i].Up
		}
		switch i {
		case 0:
			k.near = a
		case 1:
			k.next, prev = a, nil
		default:
			prev = c.internArgsKey(a)
		}
	}
	return k
}

// chain returns k as a single interned chain.
func (c *OpContext) chain(k callKey) *argsKey {
	if k.near == (argsKey{}) {
		return nil
	}
	if k.next != (argsKey{}) {
		k.near.prev = c.internArgsKey(k.next)
	}
	return c.internArgsKey(k.near)
}

// callerKey returns the key that identifies env itself: its [funcEnvKey].
func (c *OpContext) callerKey(env *Environment) argsKey {
	k := env.funcKey(c)
	return argsKey{envIdent: envIdent{
		vertex: k.vertex,
		up:     k.up,
		label:  k.label,
		level:  unknownReads,
	}}
}

// envIdent returns the identity of what references into e read.
func (c *OpContext) envIdent(e *Environment) envIdent {
	switch v := e.Vertex; v.binds {
	case bindsIteration:
		var key Feature
		for _, a := range v.Arcs {
			switch x := a.BaseValue.(type) {
			case *Vertex:
				return envIdent{vertex: c.derefRoot(x)}
			case *String:
				key = c.StringLabel(x.Str)
			case *Num:
				if i, err := x.X.Int64(); err == nil {
					key = MakeIntLabel(IntLabel, i)
				}
			}
		}
		if key != InvalidLabel && len(v.Arcs) == 1 {
			// Only the key is bound: the label of the iterated arc.
			return envIdent{label: key}
		}
	case bindsValue:
		if len(v.Arcs) == 1 && len(v.Arcs[0].Conjuncts) == 1 {
			x := v.Arcs[0].Conjuncts[0]
			if expr := x.Expr(); expr != nil {
				r := c.exprReads(expr)
				return envIdent{expr: r, bound: c.chain(c.readsKey(r, x.Env))}
			}
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

// exprReads returns, once per expression, the environments that an
// expression bound by a let or try clause reads. The result is unique to the
// expression, so that its pointer identifies it.
func (c *OpContext) exprReads(x Expr) *envReads {
	if r, ok := c.envReads[x]; ok {
		return r
	}
	r := &envReads{}
	readsOf(x, r)
	r.finish()
	if c.envReads == nil {
		c.envReads = map[Expr]*envReads{}
	}
	c.envReads[x] = r
	return r
}

// argReads returns the environments that the arguments of call read.
func argReads(call *CallExpr) *envReads {
	r := &envReads{}
	for _, a := range call.Args {
		readsOf(a, r)
	}
	r.finish()
	return r
}

func (r *envReads) finish() {
	slices.Sort(r.levels)
	r.levels = slices.Compact(r.levels)
	slices.Sort(r.exact)
	r.exact = slices.Compact(r.exact)
}

// readsOf adds to r the environment levels that the free references of x,
// evaluated at level 0, read, using the walk over a function's escaping
// references (see [walkEscapes]). A label reference reads the label of its
// environment, which [envIdent] includes; a let reference reads a value
// that may be cached per environment, so its level is recorded as exact. A
// construct that walkEscapes does not model sets r.unknown.
func readsOf(x Node, r *envReads) {
	ok := walkEscapes(x, 0, func(ref Node, up int32) bool {
		// A walk may pass the selection a.b.c for the reference a it
		// selects from.
		for {
			sel, ok := ref.(*SelectorExpr)
			if !ok {
				break
			}
			ref = sel.X
		}
		switch ref := ref.(type) {
		case *FieldReference, *LabelReference:
			r.levels = append(r.levels, up)
		case *LetReference:
			r.levels = append(r.levels, up)
			r.exact = append(r.exact, up)
		case *ValueReference:
			// A value reference counts one more level than the scope it
			// refers to; a count of zero is the vertex being evaluated.
			if ref.UpCount == 0 || up == 0 {
				return false
			}
			r.levels = append(r.levels, up-1)
		default:
			return false
		}
		return true
	})
	if !ok {
		r.unknown = true
	}
}
