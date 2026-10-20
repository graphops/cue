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
	"testing"

	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/runtime"
	"cuelang.org/go/internal/core/walk"
)

// TestArgLevel checks the shallowest environment level that the arguments of
// the call to F in a field x read, relative to the struct holding x. The memo
// of function call results is keyed by that ancestor environment, so a level
// that is too deep would share results between calls whose arguments differ.
func TestArgLevel(t *testing.T) {
	const none = adt.NoFreeRef
	testCases := []struct {
		name string
		src  string // declares x: F(...) somewhere
		want int32
	}{
		{"sibling", `s: {n: 1, x: F(n)}`, 0},
		{"self", `s: {x: F(self)}`, 0},
		{"parent", `n: 1, s: {x: F(n)}`, 1},
		{"grandparent", `n: 1, s: t: {x: F(n)}`, 2},
		{"constant", `s: {x: F(1)}`, none},
		{"builtin", `s: {x: F(len)}`, none},
		{"mixed", `n: 1, s: {m: 2, x: F(n, m)}`, 0},

		// Scopes opened by the argument bind the references inside.
		{"structLit", `n: 1, s: {x: F({a: n, b: a})}`, 1},
		{"structSelf", `s: {x: F({a: self})}`, none},
		{"listLit", `n: 1, s: {x: F([n, n])}`, 1},
		{"nestedStruct", `s: {m: 1, x: F({a: {b: m}})}`, 0},
		{"structLet", `n: 1, s: {x: F({let L = n, a: L})}`, 1},
		{"forClause", `n: [1], s: {x: F([for v in n {v}])}`, 1},
		{"forSource", `s: {n: [1], x: F([for v in n {v}])}`, 0},
		{"letClause", `n: 1, s: {x: F([for v in [1] let w = v + n {w}])}`, 1},
		{"tryClause", `n: {a: 1}, s: {x: F({try A = n.a? {b: A}})}`, 1},
		{"ifClause", `n: 1, s: {x: F({if n > 0 {a: n}})}`, 1},
		{"elseClause", `n: 1, s: {m: 2, x: F({if n > 0 {a: 1} else {a: m}})}`, 0},
		{"funcLit", `n: 1, s: {x: F(func(y: int) -> int: y + n)}`, 1},
		{"funcLitParam", `s: {m: int, x: F(func(y: m) -> int: y)}`, 0},
		{"dynamicField", `n: "a", s: {x: F({(n): 1})}`, 1},
		{"pattern", `n: string, s: {x: F({[n]: 1})}`, 1},
		{"interpolation", `n: 1, s: {m: "a", x: F("\(m)-\(n)")}`, 0},
		{"labelAlias", `s: {x: F({a~(K,_): K})}`, none},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			src := "@experiment(functions)\n@experiment(aliasv2,try)\nF: func(a: _, b?: _): a\n" + tc.src
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
			if got := adt.ArgLevel(call); got != tc.want {
				t.Errorf("got level %d, want %d", got, tc.want)
			}
		})
	}
}
