// Copyright 2019 CUE Authors
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

// Package struct defines utilities for struct types.
//
//cue:pure
package structs

import (
	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/token"
	"cuelang.org/go/internal/core/adt"
	"cuelang.org/go/internal/pkg"
	"cuelang.org/go/internal/value"
)

// MinFields validates the minimum number of fields that are part of a struct.
// It can only be used as a validator, for instance `MinFields(3)`.
//
// Only fields that are part of the data model count. This excludes hidden
// fields, optional fields, and definitions.
func MinFields(object pkg.Struct, n int) (pkg.Validator, error) {
	count := object.Len()
	code := adt.EvalError
	if object.IsOpen() || count+object.NumConstraintFields() >= n {
		code = adt.IncompleteError
	}
	if count < n {
		return false, pkg.ValidationError{B: &adt.Bottom{
			Code: code,
			Err:  errors.Newf(token.NoPos, "len(fields) < MinFields(%[2]d) (%[1]d < %[2]d)", count, n),
		}}
	}
	return true, nil
}

// MaxFields validates the maximum number of fields that are part of a struct.
// It can only be used as a validator, for instance `MaxFields(3)`.
//
// Only fields that are part of the data model count. This excludes hidden
// fields, optional fields, and definitions.
func MaxFields(object pkg.Struct, n int) (pkg.Validator, error) {
	count := object.Len()
	if count > n {
		return false, pkg.ValidationError{B: &adt.Bottom{
			Code: adt.EvalError,
			Err:  errors.Newf(token.NoPos, "len(fields) > MaxFields(%[2]d) (%[1]d > %[2]d)", count, n),
		}}
	}

	return true, nil
}

// Data returns v as data: its regular fields only, with every default
// resolved and without closedness, definitions, hidden or optional fields.
// It fails, as encoding/json.Marshal would, when v is not concrete.
//
// It is the in-CUE equivalent of json.Unmarshal(json.Marshal(v)), without
// the conversion: CUE kinds such as bytes and integers keep their types, and
// the result is built from the evaluated value as fresh literals, so unifying
// it elsewhere carries none of v's defaults or constraints.
//
// An error keeps its code: a value that is only incomplete makes the call
// incomplete, not an error, so that it can still be resolved later and does
// not fail a field nobody asks to be concrete.
func Data(ctx *adt.OpContext, v cue.Value) (adt.Expr, error) {
	_, x := value.ToInternal(v)
	e, b := dataLiteral(ctx, x)
	if b != nil {
		return b, nil
	}
	return e, nil
}

// dataLiteral returns v as a literal expression, or the error, as a Bottom
// with its original code, that keeps v from being data.
func dataLiteral(ctx *adt.OpContext, v *adt.Vertex) (adt.Expr, *adt.Bottom) {
	v = v.Default()
	v.Finalize(ctx)
	switch x := v.BaseValue.(type) {
	case *adt.Bottom:
		return nil, x
	case *adt.ListMarker:
		list := &adt.ListLit{}
		for _, a := range v.Arcs {
			if !a.Label.IsInt() || !a.IsDefined(ctx) {
				continue
			}
			e, b := dataLiteral(ctx, a)
			if b != nil {
				return nil, b
			}
			list.Elems = append(list.Elems, e)
		}
		return list, nil
	case *adt.StructMarker:
		st := &adt.StructLit{}
		for _, a := range v.Arcs {
			if !a.Label.IsRegular() || a.ArcType != adt.ArcMember || !a.IsDefined(ctx) {
				continue
			}
			e, b := dataLiteral(ctx, a)
			if b != nil {
				return nil, b
			}
			st.Decls = append(st.Decls, &adt.Field{Label: a.Label, Value: e})
		}
		return st, nil
	case *adt.String, *adt.Bytes, *adt.Num, *adt.Bool, *adt.Null:
		return x.(adt.Value), nil
	}
	return nil, &adt.Bottom{
		Code: adt.IncompleteError,
		Err:  errors.Newf(token.NoPos, "incomplete value %v: struct.Data needs a concrete value", ctx.Str(v)),
	}
}
