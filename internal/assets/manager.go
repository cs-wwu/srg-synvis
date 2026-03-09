package assets

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/cs-wwu/srg-synvis/core/spatial"
)

// assetManager is the main implementation of the AssetManager interface.
type assetManager struct {
	basePath string    // Base directory for map files
	cache    *mapCache // LRU cache for loaded maps

	// Statistics (atomic for thread safety)
	cacheHits   atomic.Int64
	cacheMisses atomic.Int64
	oceanTiles  atomic.Int64
}

// NewAssetManager creates a new asset manager.
// basePath should point to the directory containing map files (e.g., "assets/maps/").
// maxMaps is the maximum number of maps to keep in memory (minimum 9 recommended).
func NewAssetManager(basePath string, maxMaps int) AssetManager {
	// Ensure basePath ends with separator

	return &assetManager{
		basePath: basePath,
		cache:    newMapCache(maxMaps),
	}
}

// LoadMapData loads both heightmap and watermap for a coordinate.
func (am *assetManager) LoadMapData(coord spatial.MapCoord) (*MapData, error) {
	// Check cache first
	if data := am.cache.Get(coord); data != nil {
		am.cacheHits.Add(1)
		return data, nil
	}

	am.cacheMisses.Add(1)

	// Try to load from disk
	mapData, err := am.loadFromDisk(coord)
	if err != nil {
		// Generate ocean if files don't exist
		if os.IsNotExist(err) {
			mapData = am.generateOceanTile(coord)
			am.oceanTiles.Add(1)
		} else {
			return nil, fmt.Errorf("failed to load map %s: %w", coord, err)
		}
	}

	// Cache the result
	am.cache.Put(mapData)

	return mapData, nil
}

// PreloadMaps loads multiple maps asynchronously.
func (am *assetManager) PreloadMaps(coords []spatial.MapCoord) <-chan LoadResult {
	results := make(chan LoadResult, len(coords))

	go func() {
		defer close(results)

		// Limit concurrent disk operations to prevent overwhelming the system
		const maxConcurrent = 3
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, coord := range coords {
			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore

			go func(c spatial.MapCoord) {
				defer wg.Done()
				defer func() { <-sem }() // Release semaphore

				data, err := am.LoadMapData(c)
				results <- LoadResult{
					Coord: c,
					Data:  data,
					Err:   err,
				}
			}(coord)
		}

		wg.Wait()
	}()

	return results
}

// IsLoaded checks if a map is currently in cache.
func (am *assetManager) IsLoaded(coord spatial.MapCoord) bool {
	return am.cache.Has(coord)
}

// GetLoadedCoords returns all currently cached coordinates.
func (am *assetManager) GetLoadedCoords() []spatial.MapCoord {
	return am.cache.GetCoords()
}

// Stats returns current cache statistics.
func (am *assetManager) Stats() CacheStats {
	cacheStats := am.cache.Stats()

	return CacheStats{
		MaxMaps:     cacheStats.MaxMaps,
		CurrentMaps: cacheStats.CurrentMaps,
		TotalMemory: cacheStats.TotalMemory,
		CacheHits:   am.cacheHits.Load(),
		CacheMisses: am.cacheMisses.Load(),
		Evictions:   cacheStats.Evictions,
		OceanTiles:  am.oceanTiles.Load(),
	}
}

func (cs CacheStats) String() string {
	return fmt.Sprintf("Cache: %d/%d maps, %d MB, Ocean Tiles: %d\nHits: %d, Misses: %d, Evictions: %d",
		cs.CurrentMaps, cs.MaxMaps, cs.TotalMemory/1024/1024, cs.CacheHits, cs.CacheMisses, cs.Evictions, cs.OceanTiles)
}

// SetMaxCached updates the maximum number of cached maps.
func (am *assetManager) SetMaxCached(max int) {
	if max < 1 {
		max = 1 // At least one map must be cacheable
	}
	am.cache.SetMaxMaps(max)
}

// Clear removes all cached data.
func (am *assetManager) Clear() {
	am.cache.Clear()
}
