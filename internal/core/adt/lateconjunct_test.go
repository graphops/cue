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
	"fmt"
	"testing"

	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/core/runtime"
)

// lateConjunctSource returns a configuration in which a comprehension yields
// n conjuncts, each a value for v that agrees with the field, into a field that
// was finalized before:
// the guard of the first comprehension reads n.a, and the second iterates over
// r, which the first adds to, so it yields into n.a once n.a is final. See
// cue/testdata/comprehensions/lateconjunct.txtar.
func lateConjunctSource(n int, v string) string {
	return fmt.Sprintf(`
@experiment(try)

import "list"

n: a: {for i in list.Range(0, %[1]d, 1) {"k\(i)": {v: i, w: "x"}}}
r: {for i in list.Range(0, %[1]d, 1) {"k\(i)": 1}}
for _, _ in r {if n.a == _|_ {r: x: 2}}
for k, _ in r {n: a: (k): v: %[2]s}
`, n, v)
}

func evalLateConjunct(tb testing.TB, size int, late string) (*adt.Vertex, *adt.OpContext) {
	tb.Helper()
	r := runtime.New()
	root, inst := r.Compile(nil, lateConjunctSource(size, late))
	if inst.Err != nil {
		tb.Fatal(inst.Err)
	}
	v := &adt.Vertex{Conjuncts: root.Conjuncts}
	ctx := adt.NewContext(r, v)
	v.Finalize(ctx)
	if err := v.Err(ctx); err != nil {
		tb.Fatal(err.Err)
	}
	return v, ctx
}

// TestLateConjunctScale checks that the cost of checking the conjuncts dropped
// for a finalized field is linear in their number. A check that unified the
// finalized field with each dropped conjunct in turn took time, and memory,
// that grew with the square of the size of the field.
//
// The cost is measured in evaluator steps rather than in time, so that the test
// does not depend on the speed of the machine.
func TestLateConjunctScale(t *testing.T) {
	const small, large = 300, 3000

	// A conjunct that is evidently an instance of the field costs what it takes
	// to evaluate it. For others, such as a bound, the conjuncts of the field
	// are evaluated again with the dropped ones once, which costs as much as the
	// field does, not as much as the field for each conjunct.
	for _, late := range []string{"_", ">=0"} {
		t.Run(late, func(t *testing.T) {
			steps := func(size int) (conjuncts, unifications int) {
				v, ctx := evalLateConjunct(t, size, late)
				n := v.Lookup(ctx.StringLabel("n")).Lookup(ctx.StringLabel("a"))
				if got := len(n.Arcs); got != size {
					t.Fatalf("n.a has %d fields, want %d", got, size)
				}
				s := ctx.Stats()
				t.Logf("n=%d: %d conjuncts, %d unifications", size, s.Conjuncts, s.Unifications)
				return int(s.Conjuncts), int(s.Unifications)
			}
			c1, u1 := steps(small)
			c2, u2 := steps(large)

			// The sizes are a factor of ten apart: allow a factor of 12, which a
			// quadratic cost, at a factor of one hundred, exceeds by far.
			const factor = 12
			if c2 > factor*c1 {
				t.Errorf("conjuncts: %d for %d, but %d for %d, more than a factor %d", c1, small, c2, large, factor)
			}
			if u2 > factor*u1 {
				t.Errorf("unifications: %d for %d, but %d for %d, more than a factor %d", u1, small, u2, large, factor)
			}
		})
	}
}

func BenchmarkLateConjunct(b *testing.B) {
	for _, late := range []string{"_", ">=0"} {
		for _, size := range []int{300, 3000} {
			b.Run(fmt.Sprintf("%s/n=%d", late, size), func(b *testing.B) {
				evalLateConjunct(b, size, late) // verify before measuring
				b.ReportAllocs()
				for b.Loop() {
					evalLateConjunct(b, size, late)
				}
			})
		}
	}
}
