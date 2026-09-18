package cty

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestCollectUnknownElementMarks(t *testing.T) {
	t.Skip("not ready yet")
	tests := []struct {
		Input      Value
		WantDirect ValueMarks
		WantNested []nestedItemMarks
	}{
		{
			Input:      EmptyObjectVal,
			WantDirect: nil,
			WantNested: nil,
		},
		{
			Input: ObjectVal(map[string]Value{
				"a": True.Mark("a"),
				"b": True.Mark("b"),
			}),
			WantDirect: NewValueMarks("a", "b"),
			WantNested: nil,
		},
		{
			Input: ObjectVal(map[string]Value{
				"a": True.Mark("a"),
				"b": ListVal([]Value{
					False.Mark("c"),
				}),
			}),
			WantDirect: NewValueMarks("a", "b"),
			WantNested: []nestedItemMarks{
				{
					key: IndexStep{
						Key: NumberIntVal(0),
					},
					marks: NewValueMarks("c"),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Input.GoString(), func(t *testing.T) {
			gotDirect, gotNested := collectUnknownElementMarks(test.Input)

			if diff := cmp.Diff(test.WantDirect, gotDirect); diff != "" {
				t.Error("wrong direct marks\n" + diff)
			}
			if diff := cmp.Diff(test.WantNested, gotNested); diff != "" {
				t.Error("wrong nested marks\n" + diff)
			}
		})
	}
}
