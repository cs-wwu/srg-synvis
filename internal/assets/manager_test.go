package assets

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestAssetManager tests the complete AssetManager implementation.
func TestAssetManager(t *testing.T) {
	// Create temporary directory for test files
	tempDir := t.TempDir()

	// Create asset manager
	am := NewAssetManager(tempDir, 3)

	// Test loading non-existent map (should generate ocean)
	t.Run("GenerateOcean", func(t *testing.T) {
		data, err := am.LoadMapData("n48w123")
		if err != nil {
			t.Fatalf("Failed to load map: %v", err)
		}

		if !data.IsOcean {
			t.Error("Expected ocean tile")
		}

		// Check dimensions
		expectedPixels := mapSize * mapSize
		if len(data.HeightData) != expectedPixels {
			t.Errorf("Invalid heightmap size: expected %d, got %d", expectedPixels, len(data.HeightData))
		}

		// Check ocean elevation
		sample := data.HeightData[100*mapSize+100] // Convert 2D to 1D indexing
		if sample != seaLevel {
			t.Errorf("Expected ocean elevation %d, got %d", seaLevel, sample)
		}

		// Check water coverage
		water := data.WaterData[100*mapSize+100]
		if water != fullWater {
			t.Errorf("Expected full water %d, got %d", fullWater, water)
		}
	})

	// Test cache hits
	t.Run("CacheHit", func(t *testing.T) {
		// First load
		data1, err := am.LoadMapData("n48w123")
		if err != nil {
			t.Fatalf("Failed to load map: %v", err)
		}

		// Second load should be from cache
		data2, err := am.LoadMapData("n48w123")
		if err != nil {
			t.Fatalf("Failed to load map: %v", err)
		}

		// Should be the same object
		if data1 != data2 {
			t.Error("Expected cached data")
		}

		stats := am.Stats()
		if stats.CacheHits == 0 {
			t.Error("Expected cache hits")
		}
	})

	// Test cache eviction
	t.Run("CacheEviction", func(t *testing.T) {
		// Load 4 maps (cache size is 3)
		coords := []string{"n48w123", "n48w122", "n47w123", "n47w122"}
		for _, coord := range coords {
			_, err := am.LoadMapData(coord)
			if err != nil {
				t.Fatalf("Failed to load %s: %v", coord, err)
			}
		}

		// First map should be evicted
		if am.IsLoaded("n48w123") {
			t.Error("Expected n48w123 to be evicted")
		}

		// Last 3 should still be loaded
		for _, coord := range coords[1:] {
			if !am.IsLoaded(coord) {
				t.Errorf("Expected %s to be loaded", coord)
			}
		}

		stats := am.Stats()
		if stats.Evictions == 0 {
			t.Error("Expected evictions")
		}
	})
}

// TestConcurrentAccess tests thread safety.
func TestConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	am := NewAssetManager(tempDir, 10)

	// Concurrent loads
	var wg sync.WaitGroup
	coords := []string{"n48w123", "n48w122", "n47w123", "n47w122", "n46w123"}

	for _, coord := range coords {
		wg.Add(1)
		go func(c string) {
			defer wg.Done()

			// Load multiple times
			for range 10 {
				_, err := am.LoadMapData(c)
				if err != nil {
					t.Errorf("Failed to load %s: %v", c, err)
				}
			}
		}(coord)
	}

	wg.Wait()

	// All maps should be loaded
	loaded := am.GetLoadedCoords()
	if len(loaded) != len(coords) {
		t.Errorf("Expected %d loaded maps, got %d", len(coords), len(loaded))
	}
}

// TestPreloadMaps tests async preloading.
func TestPreloadMaps(t *testing.T) {
	tempDir := t.TempDir()
	am := NewAssetManager(tempDir, 10)

	coords := []string{"n48w123", "n48w122", "n47w123", "invalid", "n46w123"}
	results := am.PreloadMaps(coords)

	successCount := 0
	errorCount := 0

	for result := range results {
		if result.Err != nil {
			errorCount++
			if result.Coord != "invalid" {
				t.Errorf("Unexpected error for %s: %v", result.Coord, result.Err)
			}
		} else {
			successCount++
			if result.Data == nil {
				t.Errorf("No data for %s", result.Coord)
			}
		}
	}

	if successCount != 4 {
		t.Errorf("Expected 4 successful loads, got %d", successCount)
	}
	if errorCount != 1 {
		t.Errorf("Expected 1 error, got %d", errorCount)
	}
}

// TestLoadFromDisk tests loading actual files.
func TestLoadFromDisk(t *testing.T) {
	tempDir := t.TempDir()

	// Create test map files
	coord := "n48w123"
	createTestMapFiles(t, tempDir, coord)

	am := NewAssetManager(tempDir, 3)

	data, err := am.LoadMapData(coord)
	if err != nil {
		t.Fatalf("Failed to load map: %v", err)
	}

	if data.IsOcean {
		t.Error("Expected real map, not ocean")
	}

	// Verify data using raw array access
	height := data.HeightData[100*mapSize+100]
	if height != 32768 {
		t.Errorf("Expected height 32768, got %d", height)
	}

	water := data.WaterData[100*mapSize+100]
	if water != 255 {
		t.Errorf("Expected water 255, got %d", water)
	}
}

// TestCoordinateValidation tests coordinate format validation.
func TestCoordinateValidation(t *testing.T) {
	tests := []struct {
		coord string
		valid bool
	}{
		{"n48w123", true},
		{"s01e000", true},
		{"n00w001", true},
		{"n90w180", true},
		{"", false},
		{"n48", false},
		{"x48w123", false},
		{"n100w123", false},
		{"n48x123", false},
		{"n48w1234", false},
	}

	for _, tt := range tests {
		t.Run(tt.coord, func(t *testing.T) {
			valid := isValidCoordinate(tt.coord)
			if valid != tt.valid {
				t.Errorf("isValidCoordinate(%q) = %v, want %v", tt.coord, valid, tt.valid)
			}
		})
	}
}

// TestMemoryManagement tests memory tracking.
func TestMemoryManagement(t *testing.T) {
	tempDir := t.TempDir()
	am := NewAssetManager(tempDir, 3)

	// Load several maps
	coords := []string{"n48w123", "n48w122", "n47w123"}
	for _, coord := range coords {
		_, err := am.LoadMapData(coord)
		if err != nil {
			t.Fatalf("Failed to load %s: %v", coord, err)
		}
	}

	stats := am.Stats()
	if stats.TotalMemory == 0 {
		t.Error("Expected non-zero memory usage")
	}

	// The file in memory should be about 3 heightmaps + 3 watermaps
	// Each heightmap is 3601x3601 pixels, 16 bits per pixel (raw uint16 array)
	// Each watermap is 3601x3601 pixels, 8 bits per pixel (raw uint8 array)
	rawWatermapSize := mapSize * mapSize      // uint8 array
	rawHeightmapSize := mapSize * mapSize * 2 // uint16 array = 2 bytes per element
	expectedMemory := (3 * rawHeightmapSize) + (3 * rawWatermapSize)
	tolerance := float64(0.1) // 10% tolerance

	if float64(stats.TotalMemory) < float64(expectedMemory)*(1-tolerance) ||
		float64(stats.TotalMemory) > float64(expectedMemory)*(1+tolerance) {
		t.Errorf("Memory usage %d != expected %d", stats.TotalMemory, expectedMemory)
	}
}

// Helper function to create test map files
func createTestMapFiles(t *testing.T, dir, coord string) {
	// Create test heightmap
	heightmap := image.NewGray16(image.Rect(0, 0, mapSize, mapSize))
	for y := range mapSize {
		for x := range mapSize {
			heightmap.SetGray16(x, y, color.Gray16{Y: 32768}) // Arbitrary height
		}
	}

	heightmapPath := filepath.Join(dir, coord+heightmapExtension)
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

	watermapPath := filepath.Join(dir, coord+watermapExtension)
	if err := saveGrayImage(watermapPath, watermap); err != nil {
		t.Fatalf("Failed to create test watermap: %v", err)
	}
}

// BenchmarkLoadMapData benchmarks map loading performance.
func BenchmarkLoadMapData(b *testing.B) {
	tempDir := b.TempDir()
	am := NewAssetManager(tempDir, 10)

	// Preload to ensure it's cached
	am.LoadMapData("n48w123")

	for b.Loop() {
		_, err := am.LoadMapData("n48w123")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConcurrentLoads benchmarks concurrent access.
func BenchmarkConcurrentLoads(b *testing.B) {
	tempDir := b.TempDir()
	am := NewAssetManager(tempDir, 20)

	coords := []string{"n48w123", "n48w122", "n47w123", "n47w122", "n46w123"}

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
