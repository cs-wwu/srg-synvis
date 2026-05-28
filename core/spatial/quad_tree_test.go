package spatial_test

import (
	"testing"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func collectLeaves[T any](qt *spatial.QuadTree[T]) (nodes []*spatial.QuadTreeNode[T]) {
	for leaf := range qt.Leaves() {
		nodes = append(nodes, leaf)
	}
	return
}

func TestNewQuadTree(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	assert.Len(t, collectLeaves(qt), 1)
}

func TestNewQuadTree_SizeMustBePowerOfTwo(t *testing.T) {
	assert.Panics(t, func() { spatial.NewQuadTree[int](3) })
}

func TestSplit_ProducesFourLeaves(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	qt.Split(root)
	assert.Len(t, collectLeaves(qt), 4)
}

func TestSplit_ChildrenHaveCorrectSize(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	children := qt.Split(root)

	for i, child := range children {
		assert.Equal(t, uint(8), child.Size(), "child[%d] size", i)
	}
}

func TestSplit_ChildrenHaveCorrectOrigins(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	children := qt.Split(root)

	// NW, NE, SW, SE
	expected := []spatial.GridCoord{
		{X: -8, Z: -8},
		{X: 0, Z: -8},
		{X: -8, Z: 0},
		{X: 0, Z: 0},
	}
	for i, child := range children {
		assert.Equal(t, expected[i], child.Origin(), "child[%d] origin", i)
	}
}

func TestSplit_ChildrenHaveParent(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	children := qt.Split(root)

	for i, child := range children {
		assert.Equal(t, root, child.Parent(), "child[%d] parent", i)
	}
}

func TestSplit_RootIsNoLongerLeaf(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	qt.Split(root)
	assert.False(t, root.IsLeaf())
}

func TestSplit_NonLeafPanics(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	qt.Split(root)
	assert.Panics(t, func() { qt.Split(root) })
}

func TestSplit_TwiceProducesSevenLeaves(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	children := qt.Split(root)
	qt.Split(children[0])
	assert.Len(t, collectLeaves(qt), 7)
}

func TestMerge_RestoresOneLeaf(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	qt.Split(root)
	qt.Merge(root)

	leaves := collectLeaves(qt)
	require.Len(t, leaves, 1)
	assert.Equal(t, root, leaves[0])
}

func TestMerge_LeafPanics(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	assert.Panics(t, func() { qt.Merge(root) })
}

func TestMerge_NonLeafChildPanics(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	children := qt.Split(root)
	qt.Split(children[0])
	assert.Panics(t, func() { qt.Merge(root) })
}

func TestRootHasNoParent(t *testing.T) {
	qt := spatial.NewQuadTree[int](16)
	root := collectLeaves(qt)[0]
	assert.Nil(t, root.Parent())
}
