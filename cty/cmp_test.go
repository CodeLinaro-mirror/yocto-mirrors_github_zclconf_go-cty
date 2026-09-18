package cty

import (
	"github.com/google/go-cmp/cmp"
)

var cmpOptions cmp.Option

var transformValueOp = cmp.Transformer("cty.transformValueForCmp", transformValueForCmp)
var transformTypeOp = cmp.Transformer("cty.transformTypeForCmp", transformTypeForCmp)

func init() {
	cmpOptions = cmp.Options{
		cmp.FilterValues(
			valuesCanCompareDeep,
			transformValueOp,
		),
		cmp.FilterValues(func(a, b Value) bool {
			return !valuesCanCompareDeep(a, b)
		}, cmp.Comparer(Value.RawEquals)),
		cmp.FilterValues(
			typesCanCompareDeep,
			transformTypeOp,
		),
		cmp.FilterValues(func(a, b Type) bool {
			return !typesCanCompareDeep(a, b)
		}, cmp.Comparer(Type.Equals)),
		cmp.Comparer(Path.Equals),
	}
}

func valuesCanCompareDeep(a, b Value) bool {
	if a == NilVal || b == NilVal {
		return false
	}
	if a.IsNull() || b.IsNull() {
		return false
	}
	if !(a.IsKnown() && b.IsKnown()) {
		return false
	}
	aTy := a.Type()
	bTy := b.Type()

	return (aTy.IsCollectionType() || aTy.IsTupleType() || aTy.IsObjectType()) &&
		(bTy.IsCollectionType() || bTy.IsTupleType() || bTy.IsObjectType())
}

func typesCanCompareDeep(a, b Type) bool {
	if a == NilType || b == NilType {
		return false
	}

	return (a.IsCollectionType() || a.IsTupleType() || a.IsObjectType()) &&
		(b.IsCollectionType() || b.IsTupleType() || b.IsObjectType())
}

func transformValueForCmp(v Value) any {
	if v == NilVal {
		return v
	}
	ty := v.Type()

	if unmarkedV, marks := v.Unmark(); len(marks) != 0 {
		ret := transformValueForCmp(unmarkedV)
		return maybeMarkedVal(ret, marks)
	}

	switch {

	case v.IsNull() || !v.IsKnown():
		return v

	case ty.IsObjectType():
		return newCtyObjectVal(v.AsValueMap())

	case ty.IsMapType():
		return newCtyMapVal(v.AsValueMap())

	case ty.IsTupleType():
		return newCtyTupleVal(v.AsValueSlice())

	case ty.IsListType():
		return newCtyListVal(v.AsValueSlice())

	case ty.IsSetType():
		return newCtySetVal(v.AsValueSlice())

	default:
		return v
	}
}

type ctyTupleVal []any

func newCtyTupleVal(elems []Value) ctyTupleVal {
	ret := make(ctyTupleVal, len(elems))
	for i, v := range elems {
		ret[i] = transformValueForCmp(v)
	}
	return ret
}

type ctyListVal []any

func newCtyListVal(elems []Value) ctyListVal {
	ret := make(ctyListVal, len(elems))
	for i, v := range elems {
		ret[i] = transformValueForCmp(v)
	}
	return ret
}

type ctySetVal []any

func newCtySetVal(elems []Value) ctySetVal {
	ret := make(ctySetVal, len(elems))
	for i, v := range elems {
		ret[i] = transformValueForCmp(v)
	}
	return ret
}

type ctyObjectVal map[string]any

func newCtyObjectVal(attrs map[string]Value) ctyObjectVal {
	ret := make(ctyObjectVal, len(attrs))
	for k, v := range attrs {
		ret[k] = transformValueForCmp(v)
	}
	return ret
}

type ctyMapVal map[string]any

func newCtyMapVal(attrs map[string]Value) ctyMapVal {
	ret := make(ctyMapVal, len(attrs))
	for k, v := range attrs {
		ret[k] = transformValueForCmp(v)
	}
	return ret
}

// transformTypeForCmp is a function suitable for use with cmp.Transformer
// on package github.com/google/go-cmp/cmp that turns cty collection and
// structural types into Go maps and slices so that cmp can understand
// .how to recursively compare them.
func transformTypeForCmp(ty Type) interface{} {
	if ty == NilType {
		return ty
	}

	switch {

	case ty.IsObjectType():
		return ctyObjectType(ty.AttributeTypes())

	case ty.IsMapType():
		return ctyMapType{ty.ElementType()}

	case ty.IsTupleType():
		return ctyTupleType(ty.TupleElementTypes())

	case ty.IsListType():
		return ctyListType{ty.ElementType()}

	case ty.IsSetType():
		return ctySetType{ty.ElementType()}

	default:
		return ty
	}
}

type ctyMarkedVal struct {
	Value any
	Marks ValueMarks
}

func maybeMarkedVal(ret any, marks ValueMarks) any {
	if len(marks) == 0 {
		return ret
	}
	return ctyMarkedVal{
		Value: ret,
		Marks: marks,
	}
}

type ctyObjectType map[string]Type

func (w ctyObjectType) ctyType() Type { return Object(w) }

type ctyTupleType []Type

func (w ctyTupleType) ctyType() Type { return Tuple(w) }

type ctyListType [1]Type

func (w ctyListType) ctyType() Type { return List(w[0]) }

type ctyMapType [1]Type

func (w ctyMapType) ctyType() Type { return Map(w[0]) }

type ctySetType [1]Type

func (w ctySetType) ctyType() Type { return Set(w[0]) }
