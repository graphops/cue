// Copyright 2025 CUE Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
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

// TestEmptyNodeInert checks that getting a node for an empty struct does not
// cause a nodeContext to be created.
func TestEmptyNodeInert(t *testing.T) {
	// We run the evaluator twice with different versions of the evaluator. Each
	// results in the use of emptyNode. Ensure that the it does not get
	// assigned a nodeContext.

	ctx := &OpContext{}

	s := emptyNode.getBareState(ctx)
	if s != nil {
		t.Fatal("expected nil state for empty struct")
	}
}

// TestAncestorFlagsAwaitParentTasks checks that a node is not considered to
// have all its child conjuncts while a parent task, such as a pushed-down
// comprehension, may still add to it, and that its children are not told that
// all their ancestors were processed.
func TestAncestorFlagsAwaitParentTasks(t *testing.T) {
	ctx := &OpContext{Version: internal.EvalV3, taskContext: schedConfig}
	parent := &Vertex{}
	child := &Vertex{Parent: parent}
	parent.Arcs = []*Vertex{child}

	p := parent.getState(ctx)
	p.parentTasks = append(p.parentTasks, &task{state: taskREADY})

	n := child.getState(ctx)
	n.processAncestors(attemptOnly)
	if p.meets(childConjunctsDone) {
		t.Error("childConjunctsDone set while a parent task is pending")
	}
	if n.meets(allAncestorsProcessed) {
		t.Error("allAncestorsProcessed signaled while a parent's parent task is pending")
	}
}
