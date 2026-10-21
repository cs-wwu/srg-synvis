package chunk

import (
	"testing"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/ungerik/go3d/float64/vec3"
	as "github.com/vulkan-go/asche"
)

// newTestManager returns a manager with no workers and a release hook that
// records instead of calling Vulkan. Chunks have a fake vertex buffer, which
// is never dereferenced, and no indices, so Render doesn't touch Vulkan either.
func newTestManager(t *testing.T) (*ChunkManager, *[]*as.Buffer) {
	t.Helper()

	var released []*as.Buffer
	c := &ChunkManager{
		retireDelay:   3,
		chunks:        spatial.NewQuadTree[renderChunk](1024 * 1024),
		splitRequests: map[*spatial.QuadTreeNode[renderChunk]]uint64{},
		requests:      make(chan splitRequest, 1024),
		completed:     make(chan completion, 64),
		stop:          make(chan struct{}),
		release: func(chunk *renderChunk) {
			if chunk.vertexBuffer != nil {
				released = append(released, chunk.vertexBuffer)
			}
			*chunk = renderChunk{}
		},
	}
	return c, &released
}

func fakeChunk() renderChunk {
	return renderChunk{vertexBuffer: &as.Buffer{}}
}

func fakeChunks() [4]renderChunk {
	return [4]renderChunk{fakeChunk(), fakeChunk(), fakeChunk(), fakeChunk()}
}

// Puts the camera far enough away that every parent wants to merge.
func (c *ChunkManager) moveCameraAway() {
	c.cameraPos = vec3.T{1e9, 0, 1e9}
}

func TestStaleCompletionIsFreed(t *testing.T) {
	c, released := newTestManager(t)

	var node *spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		node = leaf
	}
	c.splitRequests[node] = 2

	chunks := fakeChunks()
	c.completed <- completion{node: node, id: 1, chunks: chunks}
	c.RenderChunks(nil)

	if !node.IsLeaf() {
		t.Error("a stale completion split the node")
	}
	if len(*released) != 4 {
		t.Errorf("released %d chunks, want 4", len(*released))
	}
}

func TestCompletionForMergedNodeIsFreed(t *testing.T) {
	c, released := newTestManager(t)

	var node *spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		node = leaf
	}
	// No entry in splitRequests: the node was merged since the request.
	c.completed <- completion{node: node, id: 1, chunks: fakeChunks()}
	c.RenderChunks(nil)

	if !node.IsLeaf() {
		t.Error("a completion with no request split the node")
	}
	if len(*released) != 4 {
		t.Errorf("released %d chunks, want 4", len(*released))
	}
}

func TestCurrentCompletionSplits(t *testing.T) {
	c, released := newTestManager(t)

	var node *spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		node = leaf
	}
	c.splitRequests[node] = 1

	chunks := fakeChunks()
	c.completed <- completion{node: node, id: 1, chunks: chunks}
	c.RenderChunks(nil)

	if node.IsLeaf() {
		t.Fatal("a current completion did not split the node")
	}
	i := 0
	for child := range node.Children() {
		if child.Value.vertexBuffer != chunks[i].vertexBuffer {
			t.Errorf("child %d has the wrong chunk", i)
		}
		i++
	}
	if len(*released) != 0 {
		t.Errorf("released %d chunks, want 0", len(*released))
	}
}

func TestMergeRetiresChildrenUntilDelay(t *testing.T) {
	c, released := newTestManager(t)
	c.moveCameraAway()

	var root *spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		root = leaf
	}
	root.Value = fakeChunk()
	children := c.chunks.Split(root)
	for _, child := range children {
		child.Value = fakeChunk()
	}

	// This frame merges, and retires the children.
	c.RenderChunks(nil)
	if !root.IsLeaf() {
		t.Fatal("the node was not merged")
	}
	if root.Value.vertexBuffer == nil {
		t.Error("the merged node lost its own chunk")
	}
	if len(c.retired) != 4 {
		t.Fatalf("%d chunks retired, want 4", len(c.retired))
	}

	// Still possibly in use by the GPU for the next retireDelay-1 frames.
	for range c.retireDelay - 1 {
		c.RenderChunks(nil)
		if len(*released) != 0 {
			t.Fatalf("freed %d chunks while the GPU might still use them", len(*released))
		}
	}
	c.RenderChunks(nil)
	if len(*released) != 4 {
		t.Errorf("released %d chunks after the delay, want 4", len(*released))
	}
	if len(c.retired) != 0 {
		t.Errorf("%d chunks still retired", len(c.retired))
	}
}

func TestMergeDropsPendingSplitRequests(t *testing.T) {
	c, _ := newTestManager(t)
	c.moveCameraAway()

	var root *spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		root = leaf
	}
	children := c.chunks.Split(root)
	c.splitRequests[children[0]] = 7

	c.RenderChunks(nil)

	if _, ok := c.splitRequests[children[0]]; ok {
		t.Error("merging left a split request for a removed node")
	}
}

func TestDestroyFreesEverything(t *testing.T) {
	c, released := newTestManager(t)
	c.moveCameraAway()

	var root *spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		root = leaf
	}
	root.Value = fakeChunk()
	for _, child := range c.chunks.Split(root) {
		child.Value = fakeChunk()
	}
	c.retired = []retiredChunk{{chunk: fakeChunk()}}
	c.completed <- completion{node: root, id: 1, chunks: fakeChunks()}

	c.Destroy()
	// 1 dormant root, 4 leaves, 1 retired, 4 in flight.
	if len(*released) != 10 {
		t.Errorf("released %d chunks, want 10", len(*released))
	}

	// A second call must not panic or free anything again.
	c.Destroy()
	if len(*released) != 10 {
		t.Errorf("second Destroy released more chunks: %d", len(*released))
	}
}
