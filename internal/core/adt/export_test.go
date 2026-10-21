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

import "strconv"

// The functions in this file export internals for the external tests of
// this package.

// ArgLevel exports argLevel for testing.
func ArgLevel(call *CallExpr) int32 { return argLevel(call) }

// NoFreeRef exports noFreeRef for testing.
const NoFreeRef = noFreeRef

// CallFreeRefs reports the free references that visitFreeRefs finds in the
// arguments of call, as "path@level", or "?" for an expression that is not
// modeled.
func CallFreeRefs(call *CallExpr, idx StringIndexer) (refs []string) {
	for _, a := range call.Args {
		visitFreeRefs(a, 0, func(path Expr, level int32) bool {
			if path == nil {
				refs = append(refs, "?")
				return true
			}
			head, labels := splitPath(path)
			var s string
			switch x := head.(type) {
			case *FieldReference:
				s = x.Label.SelectorString(idx)
			case *LetReference:
				s = "let " + x.Label.SelectorString(idx)
			case *ValueReference:
				s = "self"
			case *ImportReference:
				s = "import " + x.ImportPath.StringValue(idx)
			}
			for _, l := range labels {
				s += "." + l.SelectorString(idx)
			}
			refs = append(refs, s+"@"+strconv.Itoa(int(level)))
			return true
		})
	}
	return refs
}

// ArgReads exports the environment levels that the arguments of call read,
// and whether they contain a node the analysis does not model.
func ArgReads(call *CallExpr) (levels []int32, unknown bool) {
	r := argReads(call)
	return r.levels, r.unknown
}
