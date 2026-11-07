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

package cue

import (
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/internal/core/adt"
)

// BuildInstanceShallow builds i as [Context.BuildInstance] does, but evaluates
// only what it takes to know the package's fields: none of them is evaluated.
// A caller that needs a few fields of a large package, such as one definition,
// reads them with [Value.LookupPathShallow], which evaluates only what they
// depend on, and lists the fields of a value with [Value.ShallowSelectors].
//
// The result is not a finalized value: errors in fields that are never looked
// up are not reported, and the methods of the returned [Value] other than
// LookupPathShallow and ShallowSelectors may evaluate the whole package, as Err
// does. ShallowSelectors on the empty path reports an error of the package
// itself, such as a conflict at its top level; an error building it is the
// returned value, as with BuildInstance.
//
// A value looked up shallowly is the same value a full build gives, but its
// syntax may differ where it refers to fields outside itself: the exporter
// inlines what is evaluated, so a reference to a field the full build
// finalized may be kept as a reference there and inlined here.
func (c *Context) BuildInstanceShallow(i *build.Instance, options ...BuildOption) Value {
	cfg := c.parseOptions(options)
	v, err := c.runtime().Build(&cfg, i)
	if err != nil {
		return c.makeError(err)
	}
	ctx := newContext(c.runtime())
	v.CompleteArcsShallow(ctx)
	adt.AddStats(ctx)
	if b := v.Bottom(); b != nil && !b.IsIncomplete() {
		return newErrValue(makeValue(c.runtime(), v, nil), b)
	}
	return makeValue(c.runtime(), v, nil)
}

// LookupPathShallow is [Value.LookupPath] that evaluates only the value at p:
// the values on the way have their fields computed, and nothing else, so what
// sits beside the path is not evaluated. The returned value is finalized, as
// LookupPath's is. A path that does not name a field of each value on the way,
// or that selects a pattern constraint, is looked up as LookupPath does.
func (v Value) LookupPathShallow(p Path) Value {
	if v.v == nil {
		return Value{}
	}
	n, parent, ok := v.shallowVertex(p)
	if !ok {
		return v.LookupPath(p)
	}
	n.Finalize(v.ctx())
	return makeValue(v.idx, n, parent)
}

// ShallowSelectors lists the fields of the value at p, definitions, hidden,
// optional and required fields included, without evaluating them: the values on
// the way and at p have their fields computed, and nothing else. Let fields are
// not listed, nor are pattern constraints. It fails if p does not name a field
// of each value on the way, or if the value at p is an error.
func (v Value) ShallowSelectors(p Path) ([]Selector, error) {
	if v.v == nil {
		return nil, errors.Newf(v.Pos(), "undefined value")
	}
	n, _, ok := v.shallowVertex(p)
	if !ok {
		return nil, errors.Newf(v.Pos(), "field not found: %v", p)
	}
	ctx := v.ctx()
	n.CompleteArcsShallow(ctx)
	deref := n.DerefValue()
	if b := deref.Bottom(); b != nil && !b.IsIncomplete() {
		return nil, v.toErr(b)
	}
	selectors := make([]Selector, 0, len(deref.Arcs))
	for _, a := range deref.Arcs {
		if a.Label.IsLet() || a.ArcType > adt.ArcOptional {
			continue
		}
		sel := featureToSel(a.Label, v.idx)
		if t := fromArcType(a.ArcType); t != 0 {
			sel = wrapConstraint(sel, t)
		}
		selectors = append(selectors, sel)
	}
	return selectors, nil
}

// shallowVertex walks p from v computing only the fields of each value on the
// way. It reports false when a selector is not a field of the value before it.
func (v Value) shallowVertex(p Path) (*adt.Vertex, *parent, bool) {
	n := v.v
	parent := v.parent_
	ctx := v.ctx()
outer:
	for _, sel := range p.path {
		switch sel.sel.(type) {
		case patternSelector, pathError:
			return nil, nil, false
		}
		f := sel.sel.feature(v.idx)
		n.CompleteArcsShallow(ctx)
		for _, a := range n.DerefValue().Arcs {
			if a.Label != f {
				continue
			}
			if a.ArcType > adt.ArcOptional || a.IsConstraint() && !sel.sel.isConstraint() {
				return nil, nil, false
			}
			parent = linkParent(parent, n, a)
			n = a
			continue outer
		}
		return nil, nil, false
	}
	return n, parent, true
}
