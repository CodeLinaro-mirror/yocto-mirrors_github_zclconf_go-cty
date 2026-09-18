package cty

import (
	"fmt"
	"iter"
)

// pathTree is a specialized collection type for associating arbitrary values
// with nodes in a tree of paths separate from any values those paths might
// apply to.
//
// This data structure is not optimized for wide or deep trees. It is
// intended only for trees of similar topology to reasonably-shaped [Value]
// structures.
//
// Although this is not enforced by type constraints, the design of this type
// assumes that T is a type whose zero value can represent the absense of
// a value, because that value is what will be reported for intermediate
// steps that don't have any explicit data of their own.
//
// This data structure is not safe for concurrent mutation. It's primarily
// intended for one-time construction and then ongoing read-only use, and in
// particular does not currently offer any means to completely remove a
// previously-set path from the tree.
type pathTree[T any] struct {
	root *pathTreeNode[T]
}

type pathTreeChild[T any] struct {
	step PathStep
	pathTreeNode[T]
}

type pathTreeNode[T any] struct {
	data     T
	children []pathTreeChild[T]
}

// newPathTree constructs a new, empty [pathTree].
func newPathTree[T any]() pathTree[T] {
	return pathTree[T]{
		root: &pathTreeNode[T]{},
	}
}

// Set associates the given data with the given path, discarding any data
// that was previously associated with that path.
//
// Any intermediate steps along the path that were not previously assigned
// any data are automatically assigned the zero value of T.
func (t pathTree[T]) Set(path Path, data T) {
	n := t.ensureNodeForPath(path)
	n.data = data
}

// Update calls the given function with the data associated with the given
// path, or the zero value if there is not currently any data at that path,
// and then overwrites that data with whatever the function returns.
//
// Any intermediate steps along the path that were not previously assigned
// any data are automatically assigned the zero value of T.
func (t pathTree[T]) Update(path Path, modify func(current T) T) {
	n := t.ensureNodeForPath(path)
	n.data = modify(n.data)
}

// Get returns the data associated with the given path, or the zero value of T
// if there is no such data.
func (t pathTree[T]) Get(path Path) T {
	ret, _ := t.GetOk(path)
	return ret
}

// GetOk returns the data associated with the given path and true, or the zero
// value of T and false if there is no such data.
func (t pathTree[T]) GetOk(path Path) (T, bool) {
	n := t.nodeAtPath(path)
	if n == nil {
		var zero T
		return zero, false
	}
	return n.data, true
}

// Subtree returns a [pathTree] that shares underlying storage with the subtree
// starting at the given path, or nil if that path is not currently represented
// in the reciever at all.
//
// Because the two trees share storage, mutations of the subtree will also
// affect the corresponding part of the parent tree and vice-versa.
func (t pathTree[T]) Subtree(path Path) *pathTree[T] {
	n := t.nodeAtPath(path)
	if n == nil {
		return nil
	}
	return &pathTree[T]{
		root: n,
	}
}

func (t pathTree[T]) nodeAtPath(path Path) *pathTreeNode[T] {
	n := t.root
Step:
	for _, step := range path {
		for i, cn := range n.children {
			if pathStepsEqual(step, cn.step) {
				n = &n.children[i].pathTreeNode
				continue Step
			}
		}
		return nil
	}
	return n
}

func (t pathTree[T]) ensureNodeForPath(path Path) *pathTreeNode[T] {
	n := t.root
Step:
	for _, step := range path {
		for i, cn := range n.children {
			if pathStepsEqual(step, cn.step) {
				n = &n.children[i].pathTreeNode
				continue Step
			}
		}
		i := len(n.children)
		n.children = append(n.children, pathTreeChild[T]{
			step: step,
		})
		n = &n.children[i].pathTreeNode
	}
	return n
}

// Along returns a sequence of the data associated with all paths from the
// root to the given path inclusive. Any paths without associated data are
// reported as the zero value of T.
//
// Note that more than one result may potentially be reported for each sub-path
// and the sub-paths are produced in an unspecified order, because IndexStep
// with an unknown key is treated as matching any other IndexStep.
func (t pathTree[T]) Along(path Path) iter.Seq2[Path, T] {
	root := t.root
	return func(yield func(Path, T) bool) {
		pathTreeAlong(root, path, 0, yield)
	}
}

func pathTreeAlong[T any](n *pathTreeNode[T], path Path, pathIdx int, yield func(Path, T) bool) bool {
	var data T
	if n != nil {
		data = n.data
	}
	if pathIdx >= len(path) {
		return true
	}
	if !yield(path[:pathIdx], data) {
		return false
	}
	nextStep := path[pathIdx]
	if n != nil {
		for i, cn := range n.children {
			if pathStepsMatch(cn.step, nextStep) {
				if !pathTreeAlong(&n.children[i].pathTreeNode, path, pathIdx+1, yield) {
					return false
				}
			}
		}
	} else {
		if !pathTreeAlong(nil, path, pathIdx+1, yield) {
			return false
		}
	}
	return true
}

// Beneath returns a sequence of the data associated with all paths beneath
// but excluding the given path.
//
// Paths that have never had any data set along them are not reported at all,
// and paths matching unknown values may be reported multiple times. Paths
// are returned in an unspecified order subject to change between calls and
// in future releases.
//
// Paths in successive iterations of the sequence may share a backing array,
// so callers must copy any path they intend to retain for use beyond one
// iteration of the loop.
func (t pathTree[T]) Beneath(path Path) iter.Seq2[Path, T] {
	return func(yield func(Path, T) bool) {
		for n := range t.nodesMatchingPath(path) {
			// We'll preallocate some capacity for additional nesting levels
			// to be added without reallocation.
			basePath := make(Path, len(path), len(path)+4)
			copy(basePath, path)
			pathTreeNodesBeneath(n, basePath, yield)
		}
	}
}

func pathTreeNodesBeneath[T any](n *pathTreeNode[T], basePath Path, yield func(Path, T) bool) bool {
	if !yield(basePath, n.data) {
		return false
	}
	for i, cn := range n.children {
		// Note that we're intentionally potentially sharing the backing array
		// with basePath here, which is okay because the "Beneath" documentation
		// warns that a caller is required to copy any path it intends to keep
		// beyond one iteration of the loop.
		subPath := append(basePath, cn.step)
		if !pathTreeNodesBeneath(&n.children[i].pathTreeNode, subPath, yield) {
			return false
		}
	}
	return true
}

func (t pathTree[T]) nodesMatchingPath(path Path) iter.Seq[*pathTreeNode[T]] {
	root := t.root
	return func(yield func(*pathTreeNode[T]) bool) {
		pathTreeNodesMatchingPath(root, path, yield)
	}
}

func pathTreeNodesMatchingPath[T any](n *pathTreeNode[T], path Path, yield func(*pathTreeNode[T]) bool) bool {
	if len(path) == 0 {
		// If we get here with an empty path then we've found a match.
		return yield(n)
	}
	// If we have a non-empty path then we'll keep digging deeper with any
	// child node that matches the next path step.
	step, subPath := path[0], path[1:]
	for i, cn := range n.children {
		if !pathStepsMatch(step, cn.step) {
			continue
		}
		if !pathTreeNodesMatchingPath(&n.children[i].pathTreeNode, subPath, yield) {
			return false
		}
	}
	return true
}

func pathStepsMatch(a, b PathStep) bool {
	return pathStepsCompare(a, b, true)
}

func pathStepsEqual(a, b PathStep) bool {
	return pathStepsCompare(a, b, false)
}

func pathStepsCompare(a, b PathStep, unknownResult bool) bool {
	switch a := a.(type) {
	case nil:
		return b == nil
	case GetAttrStep:
		// GetAttrStep is comparable
		return a == b
	case IndexStep:
		// IndexStep is not comparable, so we have a little more work to do.
		b, ok := b.(IndexStep)
		if !ok {
			return false
		}
		eq := a.Key.Equals(b.Key)
		eq, _ = eq.Unmark()
		if !eq.IsKnown() {
			return unknownResult
		}
		return eq.True()
	default:
		// Should not get here; the above cases should be exhaustive
		panic(fmt.Sprintf("unsupported PathStep type %T", a))
	}
}
