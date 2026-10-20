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
	"testing"

	"cuelang.org/go/internal"
)

// TestProcessAncestorsUninitializedParent checks that processing the ancestors
// of a node initializes a parent whose nodeContext exists but was never
// initialized, rather than querying its scheduler. Such a bare nodeContext is
// created, for instance, when a conjunct is inserted into a vertex whose
// earlier nodeContext was reclaimed while it was still being evaluated.
func TestProcessAncestorsUninitializedParent(t *testing.T) {
	ctx := &OpContext{Version: internal.EvalV3, taskContext: schedConfig}
	parent := &Vertex{status: evaluating}
	child := &Vertex{Parent: parent}
	parent.Arcs = []*Vertex{child}

	if s := parent.getBareState(ctx); s == nil || s.isInitialized {
		t.Fatal("expected a bare, uninitialized parent state")
	}
	n := child.getState(ctx)
	n.processAncestors(attemptOnly)
	if !parent.state.isInitialized {
		t.Error("parent state was not initialized")
	}
}
