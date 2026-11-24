// Copyright 2021 CUE Authors
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

// Comprehension algorithm
//
// Comprehensions are expanded for, if, and let clauses that yield 0 or more
// structs to be embedded in the enclosing list or struct.
//
// CUE allows cascading of insertions, as in:
//
//     a?: int
//     b?: int
//     if a != _|_ {
//         b: 2
//     }
//     if b != _|_ {
//         c: 3
//         d: 4
//     }
//
// even though CUE does not allow the result of a comprehension to depend
// on another comprehension within a single struct. The way this works is that
// for fields with a fixed prefix path in a comprehension value, the
// comprehension is assigned to these respective fields.
//
// More concretely, the above example is rewritten to:
//
//    a?: int
//    b: if a != _|_ { 2 }
//    c: if b != _|_ { 3 }
//    d: if b != _|_ { 4 }
//
// where the fields with if clause are only inserted if their condition
// resolves to true. (Note that this is not valid CUE; it may be in the future.)
//
// With this rewrite, any dependencies in comprehension expressions will follow
// the same rules, more or less, as with normal evaluation.
//
// Note that a single comprehension may be distributed across multiple fields.
// The evaluator will ensure, however, that a comprehension is only evaluated
// once.
//
//
// Closedness
//
// The comprehension algorithm uses the usual closedness mechanism for marking
// fields that belong to a struct: it adds the StructLit associated with the
// comprehension value to the respective arc.
//
// One noteworthy point is that the fields of a struct are only legitimate for
// actual results. For instance, if an if clause evaluates to false, the
// value is not embedded.
//
// To account for this, the comprehension algorithm relies on the fact that
// the closedness information is computed as a separate step. So even if
// the StructLit is added early, its fields will only count once it is
// initialized, which is only done when at least one result is added.
//

// envYield defines a comprehension for a specific field within a comprehension
// value.
type envYield struct {
	comp *Comprehension // The comprehension being evaluated.
	env  *Environment   // The adjusted Environment.
	id   CloseInfo      // CloseInfo for the field.

	// envs accumulates the yielded environments for this firing. Reset at
	// the start of every [nodeContext.processComprehension] call.
	envs []*Environment
}

// addEnv is used as a [YieldFunc] so that we don't need to allocate a closure
// per comprehension firing to capture envs.
func (d *envYield) addEnv(env *Environment) {
	d.envs = append(d.envs, env)
}

// insertComprehension registers a comprehension with a node, possibly pushing
// down its evaluation to the node's children. It will only evaluate one level
// of fields at a time.
func (n *nodeContext) insertComprehension(
	env *Environment,
	c *Comprehension,
	ci CloseInfo,
) {
	n.scheduleTask(handleComprehension, env, c, ci)
}

type compState struct {
	ctx *OpContext
	// compID identifies this comprehension firing. Yielders that create
	// fresh Environments stamp it on them so toposort can group sibling
	// body decls (see [StructInfo.CompID]). Yielders that propagate an
	// existing env (e.g. [IfClause]) leave its CompID untouched: when
	// inherited from an upstream for/let it propagates naturally; when
	// no upstream set it (e.g. an if-only comp), no grouping is needed.
	compID uint32
	comp   *Comprehension
	i      int
	f      YieldFunc
	state  vertexStatus
}

// spawn creates a fresh Environment for the next clause in this firing,
// tagged with the firing's CompID.
func (s *compState) spawn(n *Vertex) *Environment {
	e := s.ctx.spawn(n)
	e.CompID = s.compID
	return e
}

// yield evaluates a Comprehension within the given Environment and calls
// f for each result.
func (c *OpContext) yield(
	node *Vertex, // errors are associated with this node
	env *Environment, // env for field for which this yield is called
	comp *Comprehension,
	state Flags,
	f YieldFunc, // called for every result
) *Bottom {
	// Allocate a fresh CompID for this firing. Yielders set it on every
	// fresh Environment they construct so toposort can group sibling body
	// decls inserted per yield (see [Environment.CompID]).
	c.nextCompID++
	s := &compState{
		ctx:    c,
		compID: c.nextCompID,
		comp:   comp,
		f:      f,
		state:  state.status,
	}
	y := comp.Clauses[0]

	saved := c.PushState(env, y.Source())
	if node != nil {
		defer c.PopArc(c.PushArc(node))
	}

	s.i++
	y.yield(s)
	s.i--

	return c.PopState(saved)
}

func (s *compState) yield(env *Environment) (ok bool) {
	c := s.ctx
	if s.i >= len(s.comp.Clauses) {
		s.f(env)
		return true
	}
	dst := s.comp.Clauses[s.i]
	saved := c.PushState(env, dst.Source())

	s.i++
	dst.yield(s)
	s.i--

	if b := c.PopState(saved); b != nil {
		c.AddBottom(b)
		return false
	}
	return !c.HasErr()
}

// processComprehension processes a single Comprehension conjunct.
// It returns an incomplete error if there was one. Fatal errors are
// processed as a "successfully" completed computation.
func (n *nodeContext) processComprehension(d *envYield, state vertexStatus) *Bottom {
	ctx := n.ctx

	// Compute environments via d.addEnv (a method bound to d, so no closure
	// is allocated for each call).
	d.envs = d.envs[:0]
	if err := ctx.yield(n.node, d.env, d.comp, Flags{
		status:    state,
		condition: allKnown,
		mode:      ignore,
	}, d.addEnv); err != nil {
		// The arcs the comprehension was pushed down to fail with it, as they
		// would if it were not pushed down. Cycles are reported by lookups of
		// those arcs.
		if err.Code != CycleError {
			c := MakeConjunct(d.env, err, d.id)
			t := ctx.current()
			for _, arc := range t.pushedTo {
				if s := arc.state; s != nil && s.holdsPushedDeps(t) {
					arc.insertConjunct(ctx, c, c.CloseInfo, arc.ArcType, false, false)
				}
			}
		}

		if err.IsIncomplete() {
			return err
		}

		// continue to collect other errors.
		n.node.state.addBottom(err)
		return nil
	}

	id := d.id
	// TODO: should we treat comprehension values as optional?
	// It seems so, but it causes some hangs.
	// id.setOptional(nil)

	// Mark the current task, always a comprehension task, as inserting so
	// that resolvers on this node are deferred until insertion completes.
	t := ctx.current()
	t.inserting = true
	defer func() { t.inserting = false }()

	body, envs := d.comp.Value, d.envs
	if len(envs) == 0 && d.comp.Fallback != nil {
		body, envs = d.comp.Fallback, []*Environment{d.env}
	}

	first := len(n.compLits)
	for _, env := range envs {
		n.scheduleConjunct(Conjunct{env, body, id}, id)
	}
	n.comprehensionDone(d.env, body, first, len(envs) > 0)

	return nil
}

// A compLit is a struct literal whose only embeddings are comprehensions, as
// in {if x {1}}. A comprehension embeds what it yields, so if none of them
// yields a value, the literal embeds nothing and is a struct, like a literal
// without embeddings. Whether they yield is known only once they have run, so
// the literal is marked as a struct when the last of them has completed.
//
// The body of a comprehension, as {if y {1}} in {for x in a {if y {1}}}, is
// not marked: a nested comprehension acts as a further clause, as in
// {for x in a if y {1}}, so if it yields nothing, the comprehension of which
// it is the body yields nothing either.
type compLit struct {
	env *Environment // the environment of the literal's comprehension tasks
	s   *StructLit
	ci  CloseInfo

	pending int  // the number of comprehensions and bodies yet to complete
	yielded bool // whether a comprehension yielded a value

	isBody bool // the literal is the body of a comprehension
	// parent is the index, plus one, of the compLit of whose comprehension
	// this literal is the body, or 0 if there is none.
	parent int
}

// comprehensionDone records that a comprehension of the struct literal with
// environment env has completed. It yielded body if yields is set, and the
// compLits from index first on were added while scheduling it.
func (n *nodeContext) comprehensionDone(env *Environment, body *StructLit, first int, yields bool) {
	p := -1
	for i, l := range n.compLits[:first] {
		if l.env == env {
			p = i
			break
		}
	}
	for i := first; i < len(n.compLits); i++ {
		if l := &n.compLits[i]; l.s == body {
			l.isBody = true
			l.parent = p + 1
			if p >= 0 {
				n.compLits[p].pending++
			}
		}
	}
	if p < 0 {
		return
	}
	// A body which is a compLit itself reports whether it yields a value
	// once its own comprehensions have completed.
	if yields && body.numComprehensionEmbeds() == 0 {
		n.compLits[p].yielded = true
	}
	n.compLitDone(p)
}

// compLitDone records that a comprehension or body of compLits[i] has
// completed, and marks the node as a struct if that completes a literal
// which is not a body and none of whose comprehensions yielded a value.
func (n *nodeContext) compLitDone(i int) {
	for {
		l := &n.compLits[i]
		if l.pending--; l.pending > 0 {
			return
		}
		switch {
		case !l.isBody:
			if !l.yielded {
				n.markStructLit(l.s, l.ci)
			}
			return
		case l.parent == 0:
			return
		}
		i = l.parent - 1
		n.compLits[i].yielded = n.compLits[i].yielded || l.yielded
	}
}

// pushDownDeps does a static analysis of the processed values and adjusts
// dependencies. Normally, a task of a current node is a requirement of that
// node to complete. However, if we have a comprehension, we may push that
// dependency down to the literal fields of its body and else clause. This
// allows for more fine-grained dependency analysis and can help breaking
// cycles that should be naturally broken to the user.
func pushDownDeps(n *nodeContext, t *task, x Node) condition {
	var completes condition

	switch x := x.(type) {
	case *Comprehension:
		completes = pushDownDeps(n, t, x.Value)
		if x.Fallback != nil {
			completes |= pushDownDeps(n, t, x.Fallback)
		}

	case *StructLit:
		// StructLits are mostly handled when they are a value of a
		// comprehension, as literal structs are usually directly inserted
		// without creating a task.

		for _, d := range x.Decls {
			switch x := d.(type) {
			case *Field:
				// Push the field's completion dependency down to the
				// child arc rather than blocking the parent's
				// fieldConjunctsKnown. The parent does not need to wait
				// for this comprehension to know its own field conjuncts;
				// only the child arc does, because the comprehension may
				// add sub-fields to the child. Resolver tasks on the
				// parent are skipped while the comprehension is running
				// (see hasRunningComp in process) to prevent them from
				// observing the child before the comprehension fires.

				arc, _ := n.getArc(x.Label, ArcPending)
				// If an arc with the same label already exists and has been
				// finalized in a prior evaluation, getBareState returns nil:
				// there is no scheduler to attach parentTasks to and no body
				// to recurse into. The comp's effects on this arc are
				// already captured via the existing finalized result, so
				// pushdown bookkeeping has nothing to do here.
				if arcState := arc.getBareState(n.ctx); arcState != nil {
					arcState.parentTasks = append(arcState.parentTasks, t)
					arcState.holdPushedDeps(t)
					pushDownDeps(arcState, t, x.Value)
				}

			case *BulkOptionalField, *Ellipsis:
				completes |= fieldConjunctsKnown
				// TODO: does not add fields, but may add field conjuncts;
				// confirm whether tracking dependencies here would yield
				// any additional precision.

			case *LetField:
				n.node.MultiLet = true
				// A let within a comprehension can only be referred to within
				// the comprehension. There is therefore no need to track its
				// own dependencies. Intentionally no pushDownDeps call here.
				// For the same reason, its arc cannot be observed from outside
				// the comprehension, so it does not count as adding field
				// conjuncts, which would keep the comprehension from being
				// pushed down.

			case *DynamicField:
				// Depending on arc type, may not contribute to concrete value.
				completes |= fieldConjunctsKnown

			case Value:
				completes |= valueKnown

			default:
				// Embeddings, other comprehensions.
				completes |= pushDownDeps(n, t, d)
			}
		}

		// TODO: handle lists? Theoretically we could iterate over fixed,
		// non-comprehension elements and adjust those dependencies as well.
		// Note that this was not done for evalv3.

	default:
		completes |= valueKnown | fieldConjunctsKnown
	}

	return completes | allTasksCompleted
}

// isPushedDownComp reports whether t is a comprehension task whose body and
// else clause were fully pushed down: [pushDownDeps] returns bare
// allTasksCompleted exactly when both consist of literal fields only. Such a
// task contributes conjuncts solely to the arcs it pre-created at scheduling
// time, registering itself in their parentTasks.
func (t *task) isPushedDownComp() bool {
	return t.run == handleComprehension && t.completes == allTasksCompleted
}
