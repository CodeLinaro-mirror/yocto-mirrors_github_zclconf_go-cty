package cty

import (
	"errors"
	"fmt"
)

// A Path is a sequence of operations to locate a nested value within a
// data structure.
//
// The empty Path represents the given item. Any PathSteps within represent
// taking a single step down into a data structure.
//
// Path has some convenience methods for gradually constructing a path,
// but callers can also feel free to just produce a slice of PathStep manually
// and convert to this type, which may be more appropriate in environments
// where memory pressure is a concern.
//
// Although a Path is technically mutable, by convention callers should not
// mutate a path once it has been built and passed to some other subsystem.
// Instead, use Copy and then mutate the copy before using it.
type Path []PathStep

// PathStep represents a single step down into a data structure, as part
// of a Path. PathStep is a closed interface, meaning that the only
// permitted implementations are those within this package.
type PathStep interface {
	pathStepSigil() pathStepImpl
	Apply(Value) (Value, error)
}

// embed pathImpl into a struct to declare it a PathStep implementation
type pathStepImpl struct{}

func (p pathStepImpl) pathStepSigil() pathStepImpl {
	return p
}

// Index returns a new Path that is the reciever with an IndexStep appended
// to the end.
//
// This is provided as a convenient way to construct paths, but each call
// will create garbage so it should not be used where memory pressure is a
// concern.
func (p Path) Index(v Value) Path {
	ret := make(Path, len(p)+1)
	copy(ret, p)
	ret[len(p)] = IndexStep{
		Key: v,
	}
	return ret
}

// IndexInt is a typed convenience method for Index.
func (p Path) IndexInt(v int) Path {
	return p.Index(NumberIntVal(int64(v)))
}

// IndexString is a typed convenience method for Index.
func (p Path) IndexString(v string) Path {
	return p.Index(StringVal(v))
}

// IndexPath is a convenience method to start a new Path with an IndexStep.
func IndexPath(v Value) Path {
	return Path{}.Index(v)
}

// IndexIntPath is a typed convenience method for IndexPath.
func IndexIntPath(v int) Path {
	return IndexPath(NumberIntVal(int64(v)))
}

// IndexStringPath is a typed convenience method for IndexPath.
func IndexStringPath(v string) Path {
	return IndexPath(StringVal(v))
}

// GetAttr returns a new Path that is the reciever with a GetAttrStep appended
// to the end.
//
// This is provided as a convenient way to construct paths, but each call
// will create garbage so it should not be used where memory pressure is a
// concern.
func (p Path) GetAttr(name string) Path {
	ret := make(Path, len(p)+1)
	copy(ret, p)
	ret[len(p)] = GetAttrStep{
		Name: name,
	}
	return ret
}

// UnknownDescendent returns a new Path that is the reciever with
// [UnknownDescendentStep] appended to the end.
//
// This is provided as a convenient way to construct paths, but each call
// will create garbage so it should not be used where memory pressure is a
// concern.
func (p Path) UnknownDescendent() Path {
	ret := make(Path, len(p)+1)
	copy(ret, p)
	ret[len(p)] = UnknownDescendentStep{}
	return ret
}

// Equals compares two [Path] values for exact equality.
func (p Path) Equals(other Path) bool {
	if len(p) != len(other) {
		return false
	}

	for i := range p {
		pv := p[i]
		switch pv := pv.(type) {
		case GetAttrStep:
			ov, ok := other[i].(GetAttrStep)
			if !ok || pv != ov {
				return false
			}
		case IndexStep:
			ov, ok := other[i].(IndexStep)
			if !ok {
				return false
			}

			if !pv.Key.RawEquals(ov.Key) {
				return false
			}
		case UnknownDescendentStep:
			_, ok := other[i].(UnknownDescendentStep)
			if !ok {
				return false
			}
		default:
			// Any invalid steps default to evaluating false.
			return false
		}
	}

	return true

}

// HasPrefix determines if the path p contains the provided prefix.
func (p Path) HasPrefix(prefix Path) bool {
	if len(prefix) > len(p) {
		return false
	}

	return p[:len(prefix)].Equals(prefix)
}

// GetAttrPath is a convenience method to start a new Path with a GetAttrStep.
func GetAttrPath(name string) Path {
	return Path{}.GetAttr(name)
}

// Apply applies each of the steps in turn to successive values starting with
// the given value, and returns the result. If any step returns an error,
// the whole operation returns an error.
func (p Path) Apply(val Value) (Value, error) {
	var err error
	for i, step := range p {
		val, err = step.Apply(val)
		if err != nil {
			return NilVal, PathError{
				error: err,
				Path:  p[:i+1],
			}
		}
	}
	return val, nil
}

// LastStep applies the given path up to the last step and then returns
// the resulting value and the final step.
//
// This is useful when dealing with assignment operations, since in that
// case the *value* of the last step is not important (and may not, in fact,
// present at all) and we care only about its location.
//
// Since LastStep applies all steps except the last, it will return errors
// for those steps in the same way as Apply does.
//
// If the path has *no* steps then the returned PathStep will be nil,
// representing that any operation should be applied directly to the
// given value.
func (p Path) LastStep(val Value) (Value, PathStep, error) {
	var err error

	if len(p) == 0 {
		return val, nil, nil
	}

	journey := p[:len(p)-1]
	val, err = journey.Apply(val)
	if err != nil {
		return NilVal, nil, err
	}
	return val, p[len(p)-1], nil
}

// Copy makes a shallow copy of the receiver. Often when paths are passed to
// caller code they come with the constraint that they are valid only until
// the caller returns, due to how they are constructed internally. Callers
// can use Copy to conveniently produce a copy of the value that _they_ control
// the validity of.
func (p Path) Copy() Path {
	ret := make(Path, len(p))
	copy(ret, p)
	return ret
}

// IndexStep is a Step implementation representing applying the index operation
// to a value, which must be of either a list, map, or set type.
//
// When describing a path through a *type* rather than a concrete value,
// the Key may be an unknown value, indicating that the step applies to
// *any* key of the given type.
//
// When indexing into a set, the Key is actually the element being accessed
// itself, since in sets elements are their own identity. Applying such an
// index step to a set will test if the key is present in the set and return
// it if so, but note that if the key contains any unknown values then the
// result is itself an unknown value because we cannot know whether the element
// is present or not.
type IndexStep struct {
	pathStepImpl
	Key Value
}

// Apply returns the value resulting from indexing the given value with
// our key value.
func (s IndexStep) Apply(val Value) (Value, error) {
	if val == NilVal || val.IsNull() {
		return NilVal, errors.New("cannot index a null value")
	}

	if valType := val.Type(); valType.IsSetType() {
		// Indexing into a set with [Value.Index] is not allowed because
		// sets don't have indices, but [Path] is often used to describe
		// the current location in a nested data structure when working
		// with functions like [Walk] or [Transform] and in that case
		// traversal into a set is represented as an IndexStep whose
		// key is the set element value itself, with the idea that a set
		// element effectively acts as its own "key" in the set.
		//
		// To make it possible to use that kind of path with [Path.Apply],
		// we have a special case here: if the index step's key is in the
		// set then we return that value.
		markedPresent := val.HasElement(s.Key)
		present, marks := markedPresent.Unmark()
		if !present.IsKnown() {
			return UnknownVal(valType.ElementType()).WithMarks(marks), nil
		}
		if present.False() {
			return NilVal, errors.New("set does not contain the requested element")
		}
		// We transfer the marks from the set here too, which is sufficient
		// because cty cannot not preserve marks deeply within a set so they
		// always aggregate onto the set as a whole during set construction.
		return s.Key.WithMarks(marks), nil
	}

	switch s.Key.Type() {
	case Number:
		if !(val.Type().IsListType() || val.Type().IsTupleType()) {
			return NilVal, errors.New("not a list type")
		}
	case String:
		if !val.Type().IsMapType() {
			return NilVal, errors.New("not a map type")
		}
	default:
		return NilVal, errors.New("key value not number or string")
	}

	// This value needs to be stripped of marks to check True(), but Index will
	// apply the correct marks for the result.
	has, _ := val.HasIndex(s.Key).Unmark()
	if !has.IsKnown() {
		return UnknownVal(val.Type().ElementType()), nil
	}
	if !has.True() {
		return NilVal, errors.New("value does not have given index key")
	}

	return val.Index(s.Key), nil
}

func (s IndexStep) GoString() string {
	return fmt.Sprintf("cty.IndexStep{Key:%#v}", s.Key)
}

// GetAttrStep is a Step implementation representing retrieving an attribute
// from a value, which must be of an object type.
type GetAttrStep struct {
	pathStepImpl
	Name string
}

// Apply returns the value of our named attribute from the given value, which
// must be of an object type that has a value of that name.
func (s GetAttrStep) Apply(val Value) (Value, error) {
	if val == NilVal || val.IsNull() {
		return NilVal, errors.New("cannot access attributes on a null value")
	}

	if !val.Type().IsObjectType() {
		return NilVal, errors.New("not an object type")
	}

	if !val.Type().HasAttribute(s.Name) {
		return NilVal, fmt.Errorf("object has no attribute %q", s.Name)
	}

	return val.GetAttr(s.Name), nil
}

func (s GetAttrStep) GoString() string {
	return fmt.Sprintf("cty.GetAttrStep{Name:%q}", s.Name)
}

// UnknownDescendentStep is a special [PathStep] type which can be reported
// downstream of an unknown value when describing characteristics that are
// assumed to apply to some or all of the as-yet-unknown values nested beneath
// that unknown value, as opposed to characteristics that are known to apply
// to only specific paths beneath the unknown values.
//
// Conceptually this [PathStep] represents the idea of lookup up an unknown
// index in a collection or an unknown attribute in an object, although its
// primary purpose is for use in the results of functions that perform recursive
// walks of arbitrary values and want to report different information at each
// path, in situations where it's not yet known exactly which sub-path certain
// information applies to.
//
// In particular this can happen when recursively searching for marks in a
// data structure that contains unknown values: for any unknown values that are
// placeholders for collection-typed or structural-typed values, the result
// may report that arbitrary descendents of that unknown value could have a
// certain mark without being able to commit to a specific nested path that
// mark would appear at.
//
// Unlike other [PathStep] types, this one can be a placeholder for one or more
// other path steps, potentially describing both the direct children and the
// indirect descendents of the value all at once.
type UnknownDescendentStep struct {
	pathStepImpl
}

// Apply always returns [cty.DynamicVal], but that value may potentially be
// marked to approximate what would happen if looking up an unknown index or
// attribute in the given value.
//
// It's only valid to call this this with values of types that can have
// descendents. For example, passing a primitive-typed value will return an
// error because values of those types never have descendents.
func (s UnknownDescendentStep) Apply(val Value) (Value, error) {
	if !typeCanHaveNestedMarks(val.Type()) {
		return NilVal, errors.New("value does not have any elements or attributes")
	}
	if val.IsNull() {
		return NilVal, errors.New("cannot access elements or attributes on a null value")
	}

	_, marks := val.UnmarkDeep()
	return DynamicVal.WithMarks(marks), nil
}

func (s UnknownDescendentStep) GoString() string {
	return "cty.UnknownDescendentStep{}"
}
