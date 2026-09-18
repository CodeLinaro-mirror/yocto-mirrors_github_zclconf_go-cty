package cty

import (
	"iter"
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPathTree(t *testing.T) {
	// The tests in this file expect results in a specific order but the
	// order of results of some functions is documented as unspecified and in
	// those cases it's okay to change the expected test output if the
	// implementation changes in a way that produces a different order.

	tree := newPathTree[string]()
	got := collectPathTreeTestItems(tree.Beneath(nil))
	want := []pathTreeTestItem[string]{
		{
			Path: nil,
			Data: "",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath root for empty tree:\n" + diff)
	}

	tree.Set(GetAttrPath("foo"), "foo data")
	tree.Set(GetAttrPath("bar").GetAttr("baz"), "baz data")
	tree.Set(IndexPath(NumberIntVal(0)), "index 0 data")
	got = collectPathTreeTestItems(tree.Beneath(nil))
	want = []pathTreeTestItem[string]{
		{
			Path: nil,
			Data: "",
		},
		{
			Path: GetAttrPath("foo"),
			Data: "foo data",
		},
		{
			Path: GetAttrPath("bar"),
			Data: "",
		},
		{
			Path: GetAttrPath("bar").GetAttr("baz"),
			Data: "baz data",
		},
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "index 0 data",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath root:\n" + diff)
	}

	got = collectPathTreeTestItems(tree.Along(GetAttrPath("bar").GetAttr("baz").GetAttr("beep")))
	want = []pathTreeTestItem[string]{
		{
			Path: nil,
			Data: "",
		},
		{
			Path: GetAttrPath("bar"),
			Data: "",
		},
		{
			Path: GetAttrPath("bar").GetAttr("baz"),
			Data: "baz data",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data along path:\n" + diff)
	}

	tree.Update(GetAttrPath("bar").GetAttr("baz"), func(current string) string {
		return current + " updated"
	})
	got = collectPathTreeTestItems(tree.Beneath(nil))
	want = []pathTreeTestItem[string]{
		{
			Path: nil,
			Data: "",
		},
		{
			Path: GetAttrPath("foo"),
			Data: "foo data",
		},
		{
			Path: GetAttrPath("bar"),
			Data: "",
		},
		{
			Path: GetAttrPath("bar").GetAttr("baz"),
			Data: "baz data updated",
		},
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "index 0 data",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath root after update to bar.baz:\n" + diff)
	}

	got = collectPathTreeTestItems(tree.Beneath(GetAttrPath("bar")))
	want = []pathTreeTestItem[string]{
		{
			Path: GetAttrPath("bar"),
			Data: "",
		},
		{
			Path: GetAttrPath("bar").GetAttr("baz"),
			Data: "baz data updated",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath bar:\n" + diff)
	}

	got = collectPathTreeTestItems(tree.Beneath(GetAttrPath("bar").GetAttr("baz")))
	want = []pathTreeTestItem[string]{
		{
			Path: GetAttrPath("bar").GetAttr("baz"),
			Data: "baz data updated",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath bar.baz:\n" + diff)
	}

	got = collectPathTreeTestItems(tree.Beneath(GetAttrPath("bar").GetAttr("baz").GetAttr("beep")))
	want = nil
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath bar.baz.beep:\n" + diff)
	}

	tree.Set(IndexPath(UnknownVal(Number)), "unknown index data")
	got = collectPathTreeTestItems(tree.Beneath(nil))
	want = []pathTreeTestItem[string]{
		{
			Path: nil,
			Data: "",
		},
		{
			Path: GetAttrPath("foo"),
			Data: "foo data",
		},
		{
			Path: GetAttrPath("bar"),
			Data: "",
		},
		{
			Path: GetAttrPath("bar").GetAttr("baz"),
			Data: "baz data updated",
		},
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "index 0 data",
		},
		{
			Path: IndexPath(UnknownVal(Number)),
			Data: "unknown index data",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath root after adding unknown index:\n" + diff)
	}

	got = collectPathTreeTestItems(tree.Beneath(IndexPath(NumberIntVal(0))))
	want = []pathTreeTestItem[string]{
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "index 0 data",
		},
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "unknown index data",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data beneath index 0:\n" + diff)
	}

	got = collectPathTreeTestItems(tree.Along(IndexPath(NumberIntVal(0)).GetAttr("blah")))
	want = []pathTreeTestItem[string]{
		{
			Path: nil,
			Data: "",
		},
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "index 0 data",
		},
		{
			Path: IndexPath(NumberIntVal(0)),
			Data: "unknown index data",
		},
	}
	if diff := cmp.Diff(want, got, cmpOptions); diff != "" {
		t.Error("wrong data along [0].blah:\n" + diff)
	}
}

type pathTreeTestItem[T any] struct {
	Path Path
	Data T
}

func collectPathTreeTestItems[T any](seq iter.Seq2[Path, T]) []pathTreeTestItem[T] {
	var ret []pathTreeTestItem[T]
	for p, d := range seq {
		ret = append(ret, pathTreeTestItem[T]{
			Path: slices.Clone(p),
			Data: d,
		})
	}
	return ret
}
