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

package cue_test

import (
	"slices"
	"testing"

	"github.com/go-quicktest/qt"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"cuelang.org/go/cue/parser"
)

const shallowSource = `
package p

#Schema: {
	name!:    string
	replicas: int | *x.defaults.replicas
	#Inner: {a?: string}
}

x: {
	defaults: replicas: 2
	broken: 1 & 2
}

_hidden: {b: 1}
opt?:    int
req!:    int

let L = 3
withLet: L

children: {
	one: {#Schema: {a: int}, out: 1 & 2}
	two: {#Schema: {b: string}}
}

[=~"^z"]: int
`

func shallowInstance(t *testing.T, src string) *build.Instance {
	t.Helper()
	f, err := parser.ParseFile("p.cue", src)
	qt.Assert(t, qt.IsNil(err))
	inst := build.NewContext().NewInstance("", nil)
	qt.Assert(t, qt.IsNil(inst.AddSyntax(f)))
	return inst
}

func syntaxOf(t *testing.T, v cue.Value) string {
	t.Helper()
	b, err := format.Node(v.Syntax(cue.All(), cue.InlineImports(true)))
	qt.Assert(t, qt.IsNil(err))
	return string(b)
}

// A shallow build reads a definition as a full build does, without the errors
// of fields nobody looked up.
func TestBuildInstanceShallowReadsADefinitionAlone(t *testing.T) {
	ctx := cuecontext.New()
	full := ctx.BuildInstance(shallowInstance(t, shallowSource))
	qt.Assert(t, qt.IsNotNil(full.Err()), qt.Commentf("the full build reports x.broken"))

	shallow := ctx.BuildInstanceShallow(shallowInstance(t, shallowSource))
	_, err := shallow.ShallowSelectors(cue.Path{})
	qt.Assert(t, qt.IsNil(err))

	for _, path := range []string{"#Schema", "children.one.#Schema", "children.two.#Schema", "#Schema.#Inner"} {
		got := shallow.LookupPathShallow(cue.ParsePath(path))
		want := full.LookupPath(cue.ParsePath(path))
		qt.Assert(t, qt.IsNil(got.Validate()), qt.Commentf("%s", path))
		qt.Assert(t, qt.IsNil(got.Subsume(want, cue.Schema())), qt.Commentf("%s", path))
		qt.Assert(t, qt.IsNil(want.Subsume(got, cue.Schema())), qt.Commentf("%s", path))
	}
	// A definition that refers to nothing outside itself exports as in a full
	// build; one that does may inline what the full build kept as a reference.
	for _, path := range []string{"children.one.#Schema", "children.two.#Schema", "#Schema.#Inner"} {
		qt.Assert(t, qt.Equals(syntaxOf(t, shallow.LookupPathShallow(cue.ParsePath(path))), syntaxOf(t, full.LookupPath(cue.ParsePath(path)))), qt.Commentf("%s", path))
	}
	replicas, ok := shallow.LookupPathShallow(cue.ParsePath("#Schema.replicas")).Default()
	qt.Assert(t, qt.IsTrue(ok))
	n, _ := replicas.Int64()
	qt.Assert(t, qt.Equals(n, int64(2)))
}

// The fields listed are the value's own, of every kind but lets and patterns.
func TestShallowSelectorsListsFieldsWithoutEvaluatingThem(t *testing.T) {
	ctx := cuecontext.New()
	shallow := ctx.BuildInstanceShallow(shallowInstance(t, shallowSource))

	root, err := shallow.ShallowSelectors(cue.Path{})
	qt.Assert(t, qt.IsNil(err))
	var names []string
	for _, sel := range root {
		names = append(names, sel.String())
	}
	slices.Sort(names)
	qt.Assert(t, qt.DeepEquals(names, []string{"#Schema", "_hidden", "children", "opt?", "req!", "withLet", "x"}))

	children, err := shallow.ShallowSelectors(cue.ParsePath("children.one"))
	qt.Assert(t, qt.IsNil(err))
	qt.Assert(t, qt.HasLen(children, 2), qt.Commentf("#Schema and out, though out conflicts"))

	_, err = shallow.ShallowSelectors(cue.ParsePath("children.three"))
	qt.Assert(t, qt.IsNotNil(err))
}

// A path that is not a field of the values on the way is looked up as
// LookupPath does, so its error is the same.
func TestLookupPathShallowFallsBackForAMissingField(t *testing.T) {
	ctx := cuecontext.New()
	shallow := ctx.BuildInstanceShallow(shallowInstance(t, shallowSource))
	full := ctx.BuildInstance(shallowInstance(t, shallowSource))
	for _, path := range []string{"children.three", "zebra", "x.defaults.missing"} {
		got := shallow.LookupPathShallow(cue.ParsePath(path))
		want := full.LookupPath(cue.ParsePath(path))
		qt.Assert(t, qt.Equals(got.Exists(), want.Exists()), qt.Commentf("%s", path))
	}
	qt.Assert(t, qt.Equals(syntaxOf(t, shallow.LookupPathShallow(cue.ParsePath("zebra"))), syntaxOf(t, full.LookupPath(cue.ParsePath("zebra")))))
}

// An error of the package itself is reported, as a full build reports it.
func TestBuildInstanceShallowReportsATopLevelConflict(t *testing.T) {
	ctx := cuecontext.New()
	shallow := ctx.BuildInstanceShallow(shallowInstance(t, "package p\n\n1\n{a: 1}\n"))
	_, err := shallow.ShallowSelectors(cue.Path{})
	qt.Assert(t, qt.IsNotNil(err))

	// A field nobody looks up does not fail the package: it is not evaluated.
	shallow = ctx.BuildInstanceShallow(shallowInstance(t, shallowSource))
	_, err = shallow.ShallowSelectors(cue.Path{})
	qt.Assert(t, qt.IsNil(err))
	qt.Assert(t, qt.IsNotNil(shallow.LookupPathShallow(cue.ParsePath("x.broken")).Err()))
}
