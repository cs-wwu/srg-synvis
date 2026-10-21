package chunk

import (
	"sync"

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
	// How many RenderChunks calls to wait before freeing a retired chunk. A
	// command buffer that recorded the chunk can still be executing until the
	// swapchain has cycled through every image, so this is the number of
	// swapchain images plus one. The render loop has no per-frame fences, so
	// this is a conservative bound rather than a guarantee.
	retireDelay uint64
	// Incremented once per RenderChunks call.
	frame uint64

	// Maps a node to the id of its in-flight split request. A completion is only
	// applied if its id still matches, so results for a node that was merged
	// (and possibly split again) since the request are discarded.
	splitRequests map[*spatial.QuadTreeNode[renderChunk]]uint64
	nextRequest   uint64
	requests      chan splitRequest
	completed     chan completion

	// Closed by Destroy to tell the workers to exit. `workers` is used to wait
	// for them.
	stop        chan struct{}
	workers     sync.WaitGroup
	destroyOnce sync.Once

	stats RenderStats
}

// RenderStats describes what the last RenderChunks call recorded.
type RenderStats struct {
	// Number of chunks drawn, which is also the number of draw calls.
	Chunks int
	// Number of indices submitted. Triangle strips, so this is also roughly the
	// number of vertices processed.
	Indices int
}

// Stats returns the stats from the last RenderChunks call.
func (c *ChunkManager) Stats() RenderStats {
	return c.stats
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

type splitRequest struct {
	node *spatial.QuadTreeNode[renderChunk]
	id   uint64
}

type completion struct {
	node   *spatial.QuadTreeNode[renderChunk]
	id     uint64
	chunks [4]renderChunk
}

// NewChunkManager creates a manager and starts its workers. `swapchainImages` is
// the number of swapchain images, which bounds how many frames can be in
// flight at once.
func NewChunkManager(assetManager assets.AssetManager, device vk.Device, memProps vk.PhysicalDeviceMemoryProperties, swapchainImages int) *ChunkManager {
	c := &ChunkManager{
		AssetManager:  assetManager,
		device:        device,
		memProps:      memProps,
		retireDelay:   uint64(swapchainImages) + 1,
		chunks:        spatial.NewQuadTree[renderChunk](1024 * 1024),
		splitRequests: map[*spatial.QuadTreeNode[renderChunk]]uint64{},
		requests:      make(chan splitRequest, 1024),
		completed:     make(chan completion, 64),
		stop:          make(chan struct{}),
	}
	for range 4 {
		c.workers.Add(1)
		go c.runWorker()
	}
	return c
}

func (c *ChunkManager) runWorker() {
	defer c.workers.Done()

	for {
		var req splitRequest
		select {
		case <-c.stop:
			return
		case req = <-c.requests:
		}
		node := req.node

		// Split the node `c` into 4, then build those 4 renderChunks, then send em back.
		chunks := [4]renderChunk{}
		for i, pos := range node.DirectChildrenPositions() {
			chunks[i] = c.buildChunk(pos, node.Size()/2)
		}

		select {
		case c.completed <- completion{node: node, id: req.id, chunks: chunks}:
		case <-c.stop:
			// Nobody will receive these, so free them here.
			for i := range chunks {
				chunks[i].free()
			}
			return
		}
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
			// The request is stale if the node was merged since (which clears its
			// entry, and a later split gets a new id), or if it's no longer a leaf.
			// These chunks never reached the GPU, so they can be freed right away.
			id, ok := c.splitRequests[completion.node]
			current := ok && id == completion.id
			if current {
				delete(c.splitRequests, completion.node)
			}
			if !current || !completion.node.IsLeaf() {
				for i := range completion.chunks {
					completion.chunks[i].free()
				}
				continue
			}

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
		delete(c.splitRequests, parent)
		for child := range parent.Children() {
			delete(c.splitRequests, child)
			c.retire(child.Value)
		}
		c.chunks.Merge(parent)
	}

	c.stats = RenderStats{}
	for leaf := range c.chunks.Leaves() {
		if leaf.Value.indexCount == 0 {
			continue
		}
		leaf.Value.Render(cmd)
		c.stats.Chunks++
		c.stats.Indices += int(leaf.Value.indexCount)
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
	for n < len(c.retired) && c.frame-c.retired[n].frame >= c.retireDelay {
		c.retired[n].chunk.free()
		n++
	}
	c.retired = c.retired[n:]
}

func (c *ChunkManager) requestSplit(leaf *spatial.QuadTreeNode[renderChunk]) {
	if _, ok := c.splitRequests[leaf]; !ok {
		id := c.nextRequest + 1
		select {
		case c.requests <- splitRequest{node: leaf, id: id}:
			c.nextRequest = id
			c.splitRequests[leaf] = id
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

// Destroy stops the workers, waits for them to exit, and then frees every
// chunk. The caller must have waited for the device to be idle. Calling it
// more than once is a no-op.
func (c *ChunkManager) Destroy() {
	c.destroyOnce.Do(c.destroy)
}

func (c *ChunkManager) destroy() {
	close(c.stop)
	c.workers.Wait()

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
