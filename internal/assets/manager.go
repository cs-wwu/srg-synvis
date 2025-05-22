package assets

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
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
	if basePath != "" && basePath[len(basePath)-1] != '/' {
		basePath += "/"
	}

	return &assetManager{
		basePath: basePath,
		cache:    newMapCache(maxMaps),
	}
}

// LoadMapData loads both heightmap and watermap for a coordinate.
func (am *assetManager) LoadMapData(coord string) (*MapData, error) {
	// Validate coordinate format
	if !isValidCoordinate(coord) {
		return nil, fmt.Errorf("invalid coordinate format: %s", coord)
	}

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
func (am *assetManager) PreloadMaps(coords []string) <-chan LoadResult {
	results := make(chan LoadResult, len(coords))

	go func() {
		defer close(results)

		// Limit concurrent disk operations to prevent overwhelming the system
		const maxConcurrent = 3
		sem := make(chan struct{}, maxConcurrent)
		var wg sync.WaitGroup

		for _, coord := range coords {
			// Skip invalid coordinates
			if !isValidCoordinate(coord) {
				results <- LoadResult{
					Coord: coord,
					Err:   fmt.Errorf("invalid coordinate format: %s", coord),
				}
				continue
			}

			wg.Add(1)
			sem <- struct{}{} // Acquire semaphore

			go func(c string) {
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
func (am *assetManager) IsLoaded(coord string) bool {
	return am.cache.Has(coord)
}

// GetLoadedCoords returns all currently cached coordinates.
func (am *assetManager) GetLoadedCoords() []string {
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

// isValidCoordinate validates coordinate format (e.g., "n48w123").
func isValidCoordinate(coord string) bool {
	if len(coord) != 7 {
		return false
	}

	// Check hemisphere indicators
	if coord[0] != 'n' && coord[0] != 's' {
		return false
	}
	if coord[3] != 'e' && coord[3] != 'w' {
		return false
	}

	// Check digits
	for i := 1; i <= 2; i++ {
		if coord[i] < '0' || coord[i] > '9' {
			return false
		}
	}
	for i := 4; i <= 6; i++ {
		if coord[i] < '0' || coord[i] > '9' {
			return false
		}
	}

	return true
}
