package renderer

import (
	"fmt"
	"image"
	"image/png"
	"os"
)

// This file should be used for chunk management, rendering, etc.

const (
	WATERMAP_EXTENSION  = ".swb.png"
	HEIGHTMAP_EXTENSION = ".hgt.png"

	CHUNKS_PER_MAP = 64 * 64
	NUM_MAPS       = 4
)

// A single ChunkManager to manage load/unloading images and chunks
type ChunkManager struct {
	Heightmaps    map[string]*image.Gray16
	Watermaps     map[string]*image.Gray
	Chunks        map[string]Chunk
	VisibleChunks []*Chunk
}

func ChunkKey(chunkX, chunkZ int) string {
	return fmt.Sprintf("%d,%d", chunkX, chunkZ)
}

type Chunk struct {
	vertices []Vertex
	indices  []Index
	X        int // Global X Position of the top-left chunk corner
	Z        int // Global Z Position of the top-left chunk corner
}

// Loads a watermap (coord.swb.png) into the ChunkManager
func (cm *ChunkManager) LoadWatermap(coord string) error {
	filepath := coord + WATERMAP_EXTENSION
	rawImg, err := loadMap(filepath)
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray)
	if !ok {
		return fmt.Errorf("watermap is not in 8-bit grayscale format")
	}

	cm.Watermaps[coord] = grayImg

	return nil
}

// Loads a heightmap (coord.hgt.png) into the ChunkManager
func (cm *ChunkManager) LoadHeightmap(coord string) error {
	filepath := coord + HEIGHTMAP_EXTENSION
	rawImg, err := loadMap(filepath)
	if err != nil {
		return fmt.Errorf("failed to load file: %w", err)
	}

	// Ensure the image is grayscale
	grayImg, ok := rawImg.(*image.Gray16)
	if !ok {
		return fmt.Errorf("heightmap is not in 16-bit grayscale format")
	}

	cm.Heightmaps[coord] = grayImg

	return nil
}

// Unloads both the water and heightmaps
func (cm *ChunkManager) UnloadMaps(coord string) {
	cm.unloadWatermap(coord)
	cm.unloadHeightmap(coord)
}

// Unloads a watermap (coord) from the ChunkManager
func (cm *ChunkManager) unloadWatermap(coord string) {
	delete(cm.Watermaps, coord)
}

// Unloads a heightmap (coord) from the ChunkManager
func (cm *ChunkManager) unloadHeightmap(coord string) {
	delete(cm.Heightmaps, coord)
}

// Loads a map and returns a png
func loadMap(filepath string) (image.Image, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to load map %v: %w", filepath, err)
	}
	defer file.Close()

	rawImg, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode map %v: %w", filepath, err)
	}

	// Verify image size
	width, height := rawImg.Bounds().Dx(), rawImg.Bounds().Dy()
	if width != length || height != length {
		return nil, fmt.Errorf("bad image bounds: (%d, %d)", width, height)
	}

	return rawImg, nil
}

// Loads a specific chunk into memory
func (cm *ChunkManager) LoadChunk(chunkX, chunkZ int) {

	// TODO: Load the chunk pointed to by this, verify that it's file exists
}

func NewChunkManager() *ChunkManager {
	return &ChunkManager{
		Heightmaps:    make(map[string]*image.Gray16, NUM_MAPS),
		Watermaps:     make(map[string]*image.Gray, NUM_MAPS),
		Chunks:        make(map[string]Chunk, NUM_MAPS*CHUNKS_PER_MAP),
		VisibleChunks: make([]*Chunk, 0),
	}
}
