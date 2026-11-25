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

// The functions in this file export internals for the external tests of
// this package.
// ArgLevel returns the nearest level that the arguments of call read: 0 when
// they contain a node the analysis does not model, and [NoFreeRef] when they
// read no environment.
func ArgLevel(call *CallExpr) int32 {
	r := argReads(call)
	switch {
	case r.unknown:
		return 0
	case len(r.levels) == 0:
		return noFreeRef
	}
	return r.levels[0]
}

// NoFreeRef exports noFreeRef for testing.
const NoFreeRef = noFreeRef

// ArgReads exports the environment levels that the arguments of call read,
// and whether they contain a node the analysis does not model.
func ArgReads(call *CallExpr) (levels []int32, unknown bool) {
	r := argReads(call)
	return r.levels, r.unknown
}

// ArgReadsExact exports the levels at which the arguments of call read a
// value cached per environment.
func ArgReadsExact(call *CallExpr) []int32 { return argReads(call).exact }
