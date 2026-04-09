package chunk

import (
	"github.com/cs-wwu/srg-synvis/internal/assets"
	"github.com/ungerik/go3d/float64/vec3"
	vk "github.com/vulkan-go/vulkan"
	"github.com/xlab/linmath"
)

type ChunkManager struct {
	AssetManager assets.AssetManager
	cameraPos    vec3.T
}

func NewChunkManager(assetManager assets.AssetManager) *ChunkManager {
	return &ChunkManager{
		AssetManager: assetManager,
	}
}

func (c *ChunkManager) UpdatePos(pos_lin linmath.Vec3) {
	// TODO: Switch to vec3.T everywhere.
	pos := vec3.T{float64(pos_lin[0]), float64(pos_lin[1]), float64(pos_lin[2])}

	c.cameraPos = pos
}

func (c *ChunkManager) RenderChunks(cmd vk.CommandBuffer) {
}

func (c *ChunkManager) Destroy() {
}
