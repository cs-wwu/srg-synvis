package chunk

import (
	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/assets"
	"github.com/ungerik/go3d/float64/vec3"
	as "github.com/vulkan-go/asche"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/linmath"
)

type ChunkManager struct {
	AssetManager assets.AssetManager
	cameraPos    vec3.T

	device   vk.Device
	memProps vk.PhysicalDeviceMemoryProperties

	chunks *spatial.QuadTree[renderChunk]

	// Chunks that were merged away and are waiting for the GPU to finish with
	// them. See retire().
	retired []retiredChunk
	// Incremented once per RenderChunks call.
	frame uint64

	splitRequests map[*spatial.QuadTreeNode[renderChunk]]struct{}
	requests      chan *spatial.QuadTreeNode[renderChunk]
	completed     chan completion
}

// The lifecycle of a renderChunk's GPU memory:
//
//		Building -> Live -> Dormant -> Live -> ... -> Retired -> Freed
//
//	  - Building: a worker is producing it. Owned by the worker, then by the
//	    `completed` channel. Not in the tree.
//	  - Live: it's on a leaf, and is drawn.
//	  - Dormant: the node was split, so it's an internal node. It isn't drawn,
//	    but we keep the buffers so that a merge is instant (the node just
//	    becomes Live again).
//	  - Retired: it was removed from the tree (its parent merged). It will never
//	    be recorded again, but already submitted frames might still read it, so
//	    it can't be freed yet.
//	  - Freed: buffers destroyed. Reached `retireDelay` frames after Retired.
//
// Only Retired -> Freed depends on the GPU. Chunks that never reached the GPU
// (a stale completion) skip Retired and are freed immediately.
type renderChunk struct {
	vertexBuffer *as.Buffer
	indexBuffer  *as.Buffer
	indexCount   uint32
}

type retiredChunk struct {
	chunk renderChunk
	frame uint64
}

// How many RenderChunks calls to wait before freeing a retired chunk. This
// needs to be at least the number of swapchain images, as a command buffer
// that recorded the chunk may still be executing for that long. The render
// loop has no per-frame fences, so this is a conservative constant rather than
// a guarantee.
const retireDelay = 8

type completion struct {
	node   *spatial.QuadTreeNode[renderChunk]
	chunks [4]renderChunk
}

func NewChunkManager(assetManager assets.AssetManager, device vk.Device, memProps vk.PhysicalDeviceMemoryProperties) *ChunkManager {
	c := &ChunkManager{
		AssetManager:  assetManager,
		device:        device,
		memProps:      memProps,
		chunks:        spatial.NewQuadTree[renderChunk](1024 * 1024),
		splitRequests: map[*spatial.QuadTreeNode[renderChunk]]struct{}{},
		requests:      make(chan *spatial.QuadTreeNode[renderChunk], 1024),
		completed:     make(chan completion, 64),
	}
	for range 4 {
		go c.runWorker()
	}
	return c
}

func (c *ChunkManager) runWorker() {
	for node := range c.requests {
		// Split the node `c` into 4, then build those 4 renderChunks, then send em back.
		chunks := [4]renderChunk{}
		for i, pos := range node.DirectChildrenPositions() {
			chunks[i] = c.buildChunk(pos, node.Size()/2)
		}
		c.completed <- completion{node: node, chunks: chunks}
	}
}

func (c *ChunkManager) UpdatePos(pos_lin linmath.Vec3) {
	// TODO: Switch to vec3.T everywhere.
	pos := vec3.T{float64(pos_lin[0]), float64(pos_lin[1]), float64(pos_lin[2])}

	c.cameraPos = pos
}

func (c *ChunkManager) RenderChunks(cmd vk.CommandBuffer) {
	c.frame++
	c.freeRetired()

outer:
	for {
		select {
		case completion := <-c.completed:
			// Double check that it's still a leaf. If not, these chunks never
			// reached the GPU, so they can be freed right away.
			if !completion.node.IsLeaf() {
				for i := range completion.chunks {
					completion.chunks[i].free()
				}
				continue
			}

			delete(c.splitRequests, completion.node)
			children := c.chunks.Split(completion.node)
			for i, child := range children {
				child.Value = completion.chunks[i]
			}
		default:
			break outer
		}
	}

	var toMerge []*spatial.QuadTreeNode[renderChunk]
	for leaf := range c.chunks.Leaves() {
		if leaf.Size() > MIN_CHUNK_SIZE && c.tooCoarse(leaf) {
			c.requestSplit(leaf)
		}

		parent := leaf.Parent()
		if parent == nil {
			continue
		}

		if c.tooFine(parent) && parent.AllChildrenAreLeaves() {
			toMerge = append(toMerge, leaf.Parent())
		}
	}
	for _, parent := range toMerge {
		if parent.IsLeaf() {
			continue
		}
		for child := range parent.Children() {
			delete(c.splitRequests, child)
			c.retire(child.Value)
		}
		c.chunks.Merge(parent)
	}

	for leaf := range c.chunks.Leaves() {
		leaf.Value.Render(cmd)
	}
}

// retire queues a chunk that has been removed from the tree to be freed once
// the GPU can no longer be using it.
func (c *ChunkManager) retire(chunk renderChunk) {
	if chunk.vertexBuffer == nil {
		return
	}
	c.retired = append(c.retired, retiredChunk{chunk: chunk, frame: c.frame})
}

// freeRetired frees everything that was retired long enough ago. `retired` is
// ordered by frame, so we only need to look at the front.
func (c *ChunkManager) freeRetired() {
	n := 0
	for n < len(c.retired) && c.frame-c.retired[n].frame >= retireDelay {
		c.retired[n].chunk.free()
		n++
	}
	c.retired = c.retired[n:]
}

func (c *ChunkManager) requestSplit(leaf *spatial.QuadTreeNode[renderChunk]) {
	if _, ok := c.splitRequests[leaf]; !ok {
		select {
		case c.requests <- leaf:
			c.splitRequests[leaf] = struct{}{}
		default:
			// Queue full, will retry next frame.
		}
	}
}

func (c *ChunkManager) tooCoarse(leaf *spatial.QuadTreeNode[renderChunk]) bool {
	d := c.distance(leaf)
	return d < 8000 && d/float64(leaf.Size()) < 2.0
}

func (c *ChunkManager) tooFine(leaf *spatial.QuadTreeNode[renderChunk]) bool {
	d := c.distance(leaf)
	return d > 10000 || c.distance(leaf)/float64(leaf.Size()) > 2.5
}

func (c *ChunkManager) distance(chunk *spatial.QuadTreeNode[renderChunk]) float64 {
	minX := float64(chunk.Origin().X)
	maxX := float64(chunk.Origin().X) + float64(chunk.Size())
	minZ := float64(chunk.Origin().Z)
	maxZ := float64(chunk.Origin().Z) + float64(chunk.Size())

	// Return 0 if inside the chunk.
	if c.cameraPos[0] >= minX && c.cameraPos[0] <= maxX && c.cameraPos[2] >= minZ && c.cameraPos[2] <= maxZ {
		return 0
	}

	center := vec3.T{(minX + maxX) / 2, 0, (minZ + maxZ) / 2}
	delta := center.Sub(&c.cameraPos)
	delta[1] /= 1000.0
	return delta.Length()
}

// Destroy frees every chunk. The caller must have waited for the device to be
// idle.
func (c *ChunkManager) Destroy() {
	close(c.requests)

	for {
		select {
		case completion := <-c.completed:
			for i := range completion.chunks {
				completion.chunks[i].free()
			}
			continue
		default:
		}
		break
	}

	for _, r := range c.retired {
		r.chunk.free()
	}
	c.retired = nil
	c.chunks.Walk(func(n *spatial.QuadTreeNode[renderChunk]) {
		n.Value.free()
	})
}

func (chunk *renderChunk) free() {
	if chunk.vertexBuffer != nil {
		chunk.vertexBuffer.Destroy()
	}
	if chunk.indexBuffer != nil {
		chunk.indexBuffer.Destroy()
	}
	*chunk = renderChunk{}
}

func (chunk *renderChunk) Render(cmd vk.CommandBuffer) {
	if chunk.indexCount == 0 {
		return
	}

	vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{chunk.vertexBuffer.Buffer}, []vk.DeviceSize{0})
	vk.CmdBindIndexBuffer(cmd, chunk.indexBuffer.Buffer, 0, vk.IndexTypeUint32)
	vk.CmdDrawIndexed(cmd, chunk.indexCount, 1, 0, 0, 0)
}
