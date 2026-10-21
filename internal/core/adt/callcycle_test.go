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

package adt_test

import (
	"slices"
	"testing"

	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/runtime"
	"cuelang.org/go/internal/core/walk"
)

// TestCallFreeRefs checks the free references, and the environment levels
// they are resolved in relative to the struct holding x, that are found in
// the arguments of the call to F in a field x. A call in a cyclic conjunct is
// postponed only if one of them reaches the cycle, so a reference that is
// missed, or resolved at the wrong level, could let a recursive call expand
// without end.
func TestCallFreeRefs(t *testing.T) {
	testCases := []struct {
		name string
		src  string // declares x: F(...) somewhere
		want []string
	}{
		{"constant", `s: {x: F(1)}`, nil},
		{"sibling", `s: {n: 1, x: F(n)}`, []string{"n@0"}},
		{"parent", `n: 1, s: {x: F(n)}`, []string{"n@1"}},
		{"selection", `n: a: 1, s: {x: F(n.a)}`, []string{"n.a@1"}},
		{"self", `s: {x: F(self)}`, []string{"self@0"}},
		{"selfSelection", `s: {y: 1, x: F(self.y)}`, []string{"self.y@0"}},
		{"import", "import \"math\"\ns: {x: F(math.Pi)}", []string{"import math.Pi@0"}},
		{"let", `let L = 1, s: {x: F(L)}`, []string{"let L@1"}},
		{"index", `n: [1], s: {i: 0, x: F(n[i])}`, []string{"n@1", "i@0"}},
		{"binary", `n: 1, s: {m: 2, x: F(n + m)}`, []string{"n@1", "m@0"}},
		{"interpolation", `n: 1, s: {m: "a", x: F("\(m)-\(n)")}`, []string{"m@0", "n@1"}},

		// Scopes opened by the argument bind the references inside.
		{"structLit", `n: 1, s: {x: F({a: n, b: a})}`, []string{"n@1"}},
		{"structSelf", `s: {x: F({a: self})}`, nil},
		{"listLit", `n: 1, s: {x: F([n, n.a])}`, []string{"n@1", "n.a@1"}},
		{"nestedStruct", `s: {m: 1, x: F({a: {b: m}})}`, []string{"m@0"}},
		{"structLet", `n: 1, s: {x: F({let L = n, a: L})}`, []string{"n@1"}},
		{"forClause", `n: [1], s: {x: F([for v in n {v}])}`, []string{"n@1"}},
		{"letClause", `n: 1, s: {x: F([for v in [1] let w = v + n {w}])}`, []string{"n@1"}},
		{"tryClause", `n: {a: 1}, s: {x: F({try A = n.a? {b: A}})}`, []string{"n.a@1"}},
		{"elseClause", `n: 1, s: {m: 2, x: F({if n > 0 {a: 1} else {a: m}})}`, []string{"n@1", "m@0"}},
		{"funcLit", `n: 1, s: {x: F(func(y: int) -> int: y + n)}`, []string{"n@1"}},
		{"funcLitParam", `s: {m: int, x: F(func(y: m) -> int: y)}`, []string{"m@0"}},
		{"dynamicField", `n: "a", s: {x: F({(n): 1})}`, []string{"n@1"}},
		{"labelAlias", `s: {x: F({a~(K,_): K})}`, nil},
		{"callSelection", `n: 1, s: {x: F(F(n).a)}`, []string{"F@1", "n@1"}},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := "@experiment(functions)\n@experiment(aliasv2,try)\n" + tc.src + "\nF: func(a: _, b?: _): a\n"
			r := runtime.New()
			root, inst := r.Compile(nil, src)
			if inst.Err != nil {
				t.Fatal(inst.Err)
			}
			var call *adt.CallExpr
			for _, c := range root.Conjuncts {
				w := walk.Visitor{Before: func(n adt.Node) bool {
					if x, ok := n.(*adt.CallExpr); ok && call == nil {
						if f, ok := x.Fun.(*adt.FieldReference); ok && f.Label.SelectorString(r) == "F" {
							call = x
						}
					}
					return true
				}}
				w.Elem(c.Elem())
			}
			if call == nil {
				t.Fatal("no call to F")
			}
			if got := adt.CallFreeRefs(call, r); !slices.Equal(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
