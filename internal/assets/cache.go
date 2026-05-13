package assets

import (
	"container/list"
	"sync"

	"github.com/cs-wwu/srg-synvis/core/spatial"
)

// mapCache implements an LRU cache for MapData.
// It is thread-safe and tracks memory usage and evictions.
type mapCache struct {
	maxMaps     int
	maps        map[spatial.MapCoord]*list.Element
	lru         *list.List
	mu          sync.RWMutex
	totalMemory int64
	evictions   int64
}

// cacheStats holds statistics for the cache.
type cacheStats struct {
	MaxMaps     int
	CurrentMaps int
	TotalMemory int64
	Evictions   int64
}

// newMapCache creates a new LRU cache for maps.
func newMapCache(maxMaps int) *mapCache {
	return &mapCache{
		maxMaps: maxMaps,
		maps:    make(map[spatial.MapCoord]*list.Element),
		lru:     list.New(),
	}
}

// Get retrieves a map from the cache and marks it as recently used.
func (c *mapCache) Get(coord spatial.MapCoord) *MapData {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.maps[coord]; ok {
		c.lru.MoveToFront(elem)
		return elem.Value.(*cacheEntry).data
	}

	return nil
}

// Put adds or updates a map in the cache.
func (c *mapCache) Put(data *MapData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Update existing entry
	if elem, ok := c.maps[data.Coord]; ok {
		oldData := elem.Value.(*cacheEntry).data
		c.totalMemory -= oldData.Size
		c.totalMemory += data.Size

		c.lru.MoveToFront(elem)
		elem.Value.(*cacheEntry).data = data
		return
	}

	// Add new entry
	elem := c.lru.PushFront(&cacheEntry{data: data})
	c.maps[data.Coord] = elem
	c.totalMemory += data.Size

	// Evict if necessary
	for c.lru.Len() > c.maxMaps {
		oldest := c.lru.Back()
		if oldest != nil {
			c.removeElement(oldest)
			c.evictions++
		}
	}
}

// Has checks if a coordinate is in the cache without updating LRU.
func (c *mapCache) Has(coord spatial.MapCoord) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, ok := c.maps[coord]
	return ok
}

// GetCoords returns all cached coordinates.
func (c *mapCache) GetCoords() []spatial.MapCoord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	coords := make([]spatial.MapCoord, 0, len(c.maps))
	for coord := range c.maps {
		coords = append(coords, coord)
	}
	return coords
}

// Stats returns cache statistics.
func (c *mapCache) Stats() cacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return cacheStats{
		MaxMaps:     c.maxMaps,
		CurrentMaps: c.lru.Len(),
		TotalMemory: c.totalMemory,
		Evictions:   c.evictions,
	}
}

// SetMaxMaps updates the maximum number of maps.
func (c *mapCache) SetMaxMaps(max int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maxMaps = max

	// Evict if over new limit
	for c.lru.Len() > c.maxMaps {
		oldest := c.lru.Back()
		if oldest != nil {
			c.removeElement(oldest)
			c.evictions++
		}
	}
}

// Clear removes all cached data.
func (c *mapCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.maps = make(map[spatial.MapCoord]*list.Element)
	c.lru = list.New()
	c.totalMemory = 0
}

// removeElement removes an element from the cache (must be called with lock held).
func (c *mapCache) removeElement(elem *list.Element) {
	entry := elem.Value.(*cacheEntry)
	delete(c.maps, entry.data.Coord)
	c.lru.Remove(elem)
	c.totalMemory -= entry.data.Size
}
