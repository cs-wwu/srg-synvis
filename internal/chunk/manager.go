package chunk

import (
	"math"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/assets"
	"github.com/ungerik/go3d/float64/vec3"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/linmath"
)

type ChunkManager struct {
	AssetManager assets.AssetManager
	cameraPos    vec3.T

	device   vk.Device
	memProps vk.PhysicalDeviceMemoryProperties

	chunks *spatial.QuadTree[renderChunk]

	splitRequests map[*spatial.QuadTreeNode[renderChunk]]struct{}
	requests      chan *spatial.QuadTreeNode[renderChunk]
	completed     chan completion
}

type renderChunk struct {
	vertexBuffer vk.Buffer
	indexBuffer  vk.Buffer
	indexCount   uint32
}

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
	for range 32 {
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
outer:
	for {
		select {
		case completion := <-c.completed:
			// Double check that it's still a leaf.
			if !completion.node.IsLeaf() {
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
		}
		c.chunks.Merge(parent)
	}

	for leaf := range c.chunks.Leaves() {
		leaf.Value.Render(cmd)
	}
}

func (c *ChunkManager) requestSplit(leaf *spatial.QuadTreeNode[renderChunk]) {
	if _, ok := c.splitRequests[leaf]; !ok {
		c.splitRequests[leaf] = struct{}{}
		c.requests <- leaf
	}
}

func (c *ChunkManager) tooCoarse(leaf *spatial.QuadTreeNode[renderChunk]) bool {
	return c.distance(leaf)/float64(leaf.Size()) < 0.1
}

func (c *ChunkManager) tooFine(leaf *spatial.QuadTreeNode[renderChunk]) bool {
	return c.distance(leaf)/float64(leaf.Size()) > 0.2
}

func (c *ChunkManager) distance(chunk *spatial.QuadTreeNode[renderChunk]) float64 {
	minX := float64(chunk.Origin().X)
	maxX := float64(chunk.Origin().X) + float64(chunk.Size())
	minZ := float64(chunk.Origin().Z)
	maxZ := float64(chunk.Origin().Z) + float64(chunk.Size())

	dx := max(minX-c.cameraPos[0], 0, c.cameraPos[0]-maxX)
	dz := max(minZ-c.cameraPos[2], 0, c.cameraPos[2]-maxZ)

	return math.Sqrt(dx*dx + dz*dz)
}

func (c *ChunkManager) Destroy() {
	close(c.requests)
}

func (chunk *renderChunk) Render(cmd vk.CommandBuffer) {
	if chunk.indexCount == 0 {
		return
	}

	vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{chunk.vertexBuffer}, []vk.DeviceSize{0})
	vk.CmdBindIndexBuffer(cmd, chunk.indexBuffer, 0, vk.IndexTypeUint32)
	vk.CmdDrawIndexed(cmd, chunk.indexCount, 1, 0, 0, 0)
}
