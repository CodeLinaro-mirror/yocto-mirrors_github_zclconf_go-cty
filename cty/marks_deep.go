package cty

import (
	"maps"
)

type nestedItemMarks struct {
	key      PathStep
	marks    ValueMarks
	children []nestedItemMarks
}

// collectUnknownElementMarks is a helper for the situation where a caller
// asks for an unknown key from a known collection and therefore we don't
// know which single element will be returned but we do still want to report
// all of the marks that result could've potentially had if the key were known.
//
// The first return value describes the marks that apply directly to the
// indexing result, which is the union of all of the marks on all of the direct
// elements of the collection.
//
// The second return value describes any marks nested the values of the
// collection so that operations like [Value.UnmarkDeep] can return them
// without also pulluting the shallow [Value.Unmark] result with those deeper
// value marks. Note that the keys of the objects in that slice are not
// necessarily unique: if two elements of the given collection have marks
// under the same nested path then those may be reported under multiple objects
// with the same key and only aggregated together if a caller subsequently
// asks a question that requires that aggregation.
func collectUnknownElementMarks(coll Value) (ValueMarks, []nestedItemMarks) {
	if !coll.CanIterateElements() {
		// Callers should use this helper only once they've checked that
		// the index operation could possibly succeed regardless of the
		// actual lookup key used. We also expect that the caller would've
		// already handled any marks on the collection itself and so passes
		// us the unmarked version of that collection value.
		panic("attempt to request unknown element marks for a non-aggregate type")
	}

	var direct ValueMarks
	var nested []nestedItemMarks
	for _, v := range coll.Elements() {
		v, marks := v.Unmark()
		if len(marks) != 0 && direct == nil {
			direct = make(ValueMarks)
		}
		maps.Copy(direct, marks)

		// If this element is itself an aggregate then we'll collect up
		// nested marks within it too.
		if v.CanIterateElements() {
			for ck, cv := range v.Elements() {
				childMarks, descendentMarks := collectUnknownElementMarks(cv)
				nested = append(nested, nestedItemMarks{
					key:      IndexStep{Key: ck},
					marks:    childMarks,
					children: descendentMarks,
				})
			}
		}
	}
	return direct, nested
}
