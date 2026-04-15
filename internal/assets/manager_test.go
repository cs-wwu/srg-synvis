package assets

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/cs-wwu/srg-synvis/core/spatial"
	"github.com/stretchr/testify/assert"
)

// TestAssetManager tests the complete AssetManager implementation.
func TestAssetManager(t *testing.T) {
	assert := assert.New(t)

	// Create temporary directory for test files
	tempDir := t.TempDir()

	// Create asset manager
	am := NewAssetManager(tempDir, 3)
	assert.NotNil(am)

	// Test loading non-existent map (should generate ocean)
	t.Run("GenerateOcean", func(t *testing.T) {
		data, err := am.LoadMapData(spatial.MapCoord{Lat: 48, Lon: -123})
		assert.NoError(err)

		assert.True(data.IsOcean)

		// Check dimensions
		expectedPixels := mapSize * mapSize
		assert.Len(data.HeightData, expectedPixels)

		// Check ocean elevation
		sample := data.HeightData[100*mapSize+100] // Convert 2D to 1D indexing
		assert.Equal(seaLevel, sample)

		// Check water coverage
		water := data.WaterData[100*mapSize+100]
		assert.EqualValues(fullWater, water)
	})

	// Test cache hits
	t.Run("CacheHit", func(t *testing.T) {
		// First load
		data1, err := am.LoadMapData(spatial.MapCoord{Lat: 48, Lon: -123})
		assert.NoError(err)

		// Second load should be from cache
		data2, err := am.LoadMapData(spatial.MapCoord{Lat: 48, Lon: -123})
		assert.NoError(err)

		// Should be the same object
		assert.Same(data1, data2)

		stats := am.Stats()
		assert.NotZero(stats.CacheHits)
	})

	// Test cache eviction
	t.Run("CacheEviction", func(t *testing.T) {
		// Load 4 maps (cache size is 3)
		coords := []spatial.MapCoord{
			{Lat: 48, Lon: -123},
			{Lat: 48, Lon: -122},
			{Lat: 47, Lon: -123},
			{Lat: 47, Lon: -122},
		}
		for _, coord := range coords {
			_, err := am.LoadMapData(coord)
			assert.NoError(err)
		}

		// First map should be evicted
		assert.False(am.IsLoaded(spatial.MapCoord{Lat: 48, Lon: -123}))

		// Last 3 should still be loaded
		for _, coord := range coords[1:] {
			assert.True(am.IsLoaded(coord))
		}

		stats := am.Stats()
		assert.NotZero(stats.Evictions)
	})
}

// TestConcurrentAccess tests thread safety.
func TestConcurrentAccess(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()
	am := NewAssetManager(tempDir, 10)

	// Concurrent loads
	var wg sync.WaitGroup
	coords := []spatial.MapCoord{
		{Lat: 48, Lon: -123},
		{Lat: 48, Lon: -122},
		{Lat: 47, Lon: -123},
		{Lat: 47, Lon: -122},
		{Lat: 46, Lon: -123},
	}

	for _, coord := range coords {
		wg.Add(1)
		go func(c spatial.MapCoord) {
			defer wg.Done()

			// Load multiple times
			for range 10 {
				_, err := am.LoadMapData(c)
				assert.NoError(err)
			}
		}(coord)
	}

	wg.Wait()

	// All maps should be loaded
	loaded := am.GetLoadedCoords()
	assert.Len(loaded, len(coords))
}

// TestPreloadMaps tests async preloading.
func TestPreloadMaps(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()
	am := NewAssetManager(tempDir, 10)

	coords := []spatial.MapCoord{
		{Lat: 48, Lon: -123},
		{Lat: 48, Lon: -122},
		{Lat: 47, Lon: -123},
		{Lat: 46, Lon: -123},
	}
	results := am.PreloadMaps(coords)

	successCount := 0
	errorCount := 0

	for result := range results {
		if result.Err != nil {
			errorCount++
			t.Errorf("Unexpected error for %s: %v", result.Coord, result.Err)
		} else {
			successCount++
			assert.NotNil(result.Data)
		}
	}

	assert.Equal(len(coords), successCount)
	assert.Equal(0, errorCount)
}

// TestLoadFromDisk tests loading actual files.
func TestLoadFromDisk(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()

	// Create test map files
	coord := spatial.MapCoord{Lat: 48, Lon: -123}
	createTestMapFiles(t, tempDir, coord)

	am := NewAssetManager(tempDir, 3)

	data, err := am.LoadMapData(coord)
	assert.NoError(err)

	assert.False(data.IsOcean)

	// Verify data using raw array access
	height := data.HeightData[100*mapSize+100]
	assert.Equal(uint16(32768), height)

	water := data.WaterData[100*mapSize+100]
	assert.Equal(uint8(255), water)
}

// TestMemoryManagement tests memory tracking.
func TestMemoryManagement(t *testing.T) {
	assert := assert.New(t)
	tempDir := t.TempDir()
	am := NewAssetManager(tempDir, 3)

	// Load several maps
	coords := []spatial.MapCoord{
		{Lat: 48, Lon: -123},
		{Lat: 48, Lon: -122},
		{Lat: 47, Lon: -123},
	}
	for _, coord := range coords {
		_, err := am.LoadMapData(coord)
		assert.NoError(err)
	}

	stats := am.Stats()
	assert.NotZero(stats.TotalMemory)

	// The file in memory should be about 3 heightmaps + 3 watermaps
	// Each heightmap is 3601x3601 pixels, 16 bits per pixel (raw uint16 array)
	// Each watermap is 3601x3601 pixels, 8 bits per pixel (raw uint8 array)
	rawWatermapSize := mapSize * mapSize      // uint8 array
	rawHeightmapSize := mapSize * mapSize * 2 // uint16 array = 2 bytes per element
	expectedMemory := (3 * rawHeightmapSize) + (3 * rawWatermapSize)
	tolerance := float64(0.1) // 10% tolerance

	assert.GreaterOrEqual(float64(stats.TotalMemory), float64(expectedMemory)*(1-tolerance))
	assert.LessOrEqual(float64(stats.TotalMemory), float64(expectedMemory)*(1+tolerance))
}

// Helper function to create test map files
func createTestMapFiles(t *testing.T, dir string, coord spatial.MapCoord) {
	// Create test heightmap
	heightmap := image.NewGray16(image.Rect(0, 0, mapSize, mapSize))
	for y := range mapSize {
		for x := range mapSize {
			heightmap.SetGray16(x, y, color.Gray16{Y: 32768}) // Arbitrary height
		}
	}

	heightmapPath := filepath.Join(dir, coord.String()+heightmapExtension)
	if err := saveGray16Image(heightmapPath, heightmap); err != nil {
		t.Fatalf("Failed to create test heightmap: %v", err)
	}

	// Create test watermap (grid pattern)
	watermap := image.NewGray(image.Rect(0, 0, mapSize, mapSize))
	for y := range mapSize {
		for x := range mapSize {
			if x%2 == 0 && y%2 == 0 {
				watermap.SetGray(x, y, color.Gray{Y: fullWater})
			} else {
				watermap.SetGray(x, y, color.Gray{Y: 0}) // Use 0 for no water
			}
		}
	}

	watermapPath := filepath.Join(dir, coord.String()+watermapExtension)
	if err := saveGrayImage(watermapPath, watermap); err != nil {
		t.Fatalf("Failed to create test watermap: %v", err)
	}
}

// BenchmarkLoadMapData benchmarks map loading performance.
func BenchmarkLoadMapData(b *testing.B) {
	tempDir := b.TempDir()
	am := NewAssetManager(tempDir, 10)

	// Preload to ensure it's cached
	am.LoadMapData(spatial.MapCoord{Lat: 48, Lon: -123})

	for b.Loop() {
		_, err := am.LoadMapData(spatial.MapCoord{Lat: 48, Lon: -123})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrentLoads benchmarks concurrent access.
func BenchmarkConcurrentLoads(b *testing.B) {
	tempDir := b.TempDir()
	am := NewAssetManager(tempDir, 20)

	coords := []spatial.MapCoord{
		{Lat: 48, Lon: -123},
		{Lat: 48, Lon: -122},
		{Lat: 47, Lon: -123},
		{Lat: 47, Lon: -122},
		{Lat: 46, Lon: -123},
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			coord := coords[i%len(coords)]
			_, err := am.LoadMapData(coord)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// saveGray16Image saves a 16-bit grayscale image as PNG.
func saveGray16Image(path string, img *image.Gray16) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	return encoder.Encode(file, img)
}

// saveGrayImage saves an 8-bit grayscale image as PNG.
func saveGrayImage(path string, img *image.Gray) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := png.Encoder{CompressionLevel: png.DefaultCompression}
	return encoder.Encode(file, img)
}
