package assets

import (
	"time"

	"github.com/cs-wwu/srg-synvis/core/spatial"
)

// MapData represents a complete map tile with elevation and water data.
// Heightmaps and watermaps are always loaded and cached together as they
// are always used together in the terrain system.
type MapData struct {
	Coord      spatial.MapCoord // Map coordinate (e.g., "n48w123")
	HeightData []uint16         // 3601 * 3601 * 2 bytes of height data
	WaterData  []uint8          // 3601 * 3601 bytes of water data
	LoadedAt   time.Time        // When this data was loaded
	Size       int64            // Approximate memory usage in bytes
	IsOcean    bool             // True if this is a generated ocean tile
}

// AssetManager handles loading and caching of map data for the terrain system.
// It provides thread-safe access to heightmaps and watermaps, with automatic
// generation of ocean tiles for missing data.
type AssetManager interface {
	// LoadMapData loads both heightmap and watermap for a coordinate.
	// Returns cached data if available, loads from disk if not.
	// Generates ocean data if files don't exist.
	LoadMapData(coord spatial.MapCoord) (*MapData, error)

	// PreloadMaps loads multiple maps asynchronously.
	// Returns a channel that receives results as they complete.
	PreloadMaps(coords []spatial.MapCoord) <-chan LoadResult

	// IsLoaded checks if a map is currently in cache.
	IsLoaded(coord spatial.MapCoord) bool

	// GetLoadedCoords returns all currently cached coordinates.
	GetLoadedCoords() []spatial.MapCoord

	// Stats returns current cache statistics.
	Stats() CacheStats

	// SetMaxCached updates the maximum number of cached maps.
	// This may trigger evictions if the cache is over the new limit.
	SetMaxCached(max int)

	// Clear removes all cached data.
	Clear()
}

// LoadResult is returned from async loading operations.
type LoadResult struct {
	Coord spatial.MapCoord // Map coordinate
	Data  *MapData         // Loaded data (nil if error)
	Err   error            // Error if loading failed
}

// CacheStats provides insight into cache performance and memory usage.
type CacheStats struct {
	MaxMaps     int   // Maximum number of maps that can be cached
	CurrentMaps int   // Number of maps currently cached
	TotalMemory int64 // Total memory used by cached maps (bytes)
	CacheHits   int64 // Number of cache hits
	CacheMisses int64 // Number of cache misses
	Evictions   int64 // Number of maps evicted from cache
	OceanTiles  int64 // Number of generated ocean tiles
}

// Internal types

// cacheEntry wraps MapData for the LRU cache
type cacheEntry struct {
	data *MapData
}

// fileType represents different asset file types
type fileType int

const (
	fileTypeHeightmap fileType = iota
	fileTypeWatermap
	fileTypeTexture
)
