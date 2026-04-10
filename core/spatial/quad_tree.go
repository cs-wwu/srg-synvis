package spatial

import "iter"

type QuadTree[T any] struct {
	root *QuadTreeNode[T]
}

type QuadTreeNode[T any] struct {
	origin   GridCoord // The top left corner.
	size     uint      // The width/height of this node. Always a power of 2.
	parent   *QuadTreeNode[T]
	children [4]*QuadTreeNode[T]

	Value T
}

func NewQuadTree[T any](size uint) *QuadTree[T] {
	if (size & (size - 1)) != 0 {
		panic("size must be a power of 2")
	}

	origin := GridCoord{-int(size) / 2, -int(size) / 2}
	root := &QuadTreeNode[T]{origin: origin, size: size}
	return &QuadTree[T]{root: root}
}

func (qt *QuadTree[T]) Leaves() iter.Seq[*QuadTreeNode[T]] {
	return func(yield func(*QuadTreeNode[T]) bool) {
		qt.root.walkLeaves(yield)
	}
}

func (n *QuadTreeNode[T]) walkLeaves(yield func(*QuadTreeNode[T]) bool) bool {
	if n.IsLeaf() {
		return yield(n)
	}
	for _, child := range n.children {
		if !child.walkLeaves(yield) {
			return false
		}
	}
	return true
}

func (n *QuadTreeNode[T]) Origin() GridCoord        { return n.origin }
func (n *QuadTreeNode[T]) Size() uint               { return n.size }
func (n *QuadTreeNode[T]) Parent() *QuadTreeNode[T] { return n.parent }
func (n *QuadTreeNode[T]) IsLeaf() bool             { return n.children[0] == nil }

func (n *QuadTreeNode[T]) Children() iter.Seq[*QuadTreeNode[T]] {
	return func(yield func(*QuadTreeNode[T]) bool) {
		for _, child := range n.children {
			if !yield(child) {
				break
			}
		}
	}
}

func (n *QuadTreeNode[T]) AllChildrenAreLeaves() bool {
	// Golang please - I just want `iter.All()`.
	for _, child := range n.children {
		if !child.IsLeaf() {
			return false
		}
	}
	return true
}

func (n *QuadTreeNode[T]) DirectChildrenPositions() [4]GridCoord {
	half := n.size / 2
	mid := GridCoord{n.origin.X + int(half), n.origin.Z + int(half)}

	return [4]GridCoord{
		n.origin,            // NW
		{mid.X, n.origin.Z}, // NE
		{n.origin.X, mid.Z}, // SW
		mid,                 // SE
	}
}

// Split subdivides a leaf node into 4 children and returns them (NW, NE, SW, SE).
// Panics if the node is not a leaf or cannot be subdivided further.
func (qt *QuadTree[T]) Split(node *QuadTreeNode[T]) [4]*QuadTreeNode[T] {
	if !node.IsLeaf() {
		panic("cannot split a non-leaf node")
	}
	if node.size < 2 {
		panic("cannot split a node of size 1")
	}

	for i, pos := range node.DirectChildrenPositions() {
		node.children[i] = &QuadTreeNode[T]{origin: pos, size: node.size / 2, parent: node}
	}

	return node.children
}

// Merge collapses a node's 4 children back into a leaf.
// Panics if the node is already a leaf.
func (qt *QuadTree[T]) Merge(node *QuadTreeNode[T]) {
	if node.IsLeaf() {
		panic("cannot merge a leaf node")
	}
	for _, child := range node.children {
		if !child.IsLeaf() {
			panic("cannot merge a node with non-leaf children")
		}
	}
	node.children = [4]*QuadTreeNode[T]{}
}
