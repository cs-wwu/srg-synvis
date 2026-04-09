package chunk

import (
	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/cs-wwu/srg-synvis/internal/assets"
	"github.com/ungerik/go3d/float64/vec3"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/linmath"
)

type ChunkManager struct {
	AssetManager assets.AssetManager
	cameraPos    vec3.T

	chunks *spatial.QuadTree[renderChunk]
}

type renderChunk struct {
	vertexBuffer vk.Buffer
	indexBuffer  vk.Buffer
	indexCount   uint32
}

func NewChunkManager(assetManager assets.AssetManager) *ChunkManager {
	return &ChunkManager{
		AssetManager: assetManager,
		chunks:       spatial.NewQuadTree[renderChunk](1024),
	}
}

func (c *ChunkManager) UpdatePos(pos_lin linmath.Vec3) {
	// TODO: Switch to vec3.T everywhere.
	pos := vec3.T{float64(pos_lin[0]), float64(pos_lin[1]), float64(pos_lin[2])}

	c.cameraPos = pos
}

func (c *ChunkManager) RenderChunks(cmd vk.CommandBuffer) {
	for leaf := range c.chunks.Leaves() {
		leaf.Value.Render(cmd)
	}
}

func (c *ChunkManager) Destroy() {
}

func (chunk *renderChunk) Render(cmd vk.CommandBuffer) {
	if chunk.indexCount == 0 {
		return
	}

	vk.CmdBindVertexBuffers(cmd, 0, 1, []vk.Buffer{chunk.vertexBuffer}, []vk.DeviceSize{0})
	vk.CmdBindIndexBuffer(cmd, chunk.indexBuffer, 0, vk.IndexTypeUint32)
	vk.CmdDrawIndexed(cmd, chunk.indexCount, 1, 0, 0, 0)
}
