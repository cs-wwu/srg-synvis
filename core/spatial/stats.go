package spatial

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// GridStats holds statistics about the grid system
type GridStats struct {
	TotalChunks      int
	VisibleChunks    int
	LoadedChunks     int
	PendingChunks    int
	ChunksByLOD      []int // Count of chunks per LOD level
	MaxNeighbors     int   // Maximum neighbors per chunk
	AverageNeighbors float32
	AverageDistance  float32
	MinDistance      float32
	MaxDistance      float32
	GridBounds       GridBounds    // Extents of the grid
	UpdateTime       time.Duration // Time since last visibility update
}

// GridBounds represents the extents of the grid
type GridBounds struct {
	MinX, MaxX int
	MinZ, MaxZ int
}

// Stats returns statistics about the grid
func (g *TerrainGrid) Stats() GridStats {
	g.mu.RLock()
	defer g.mu.RUnlock()

	stats := GridStats{
		TotalChunks:   len(g.chunks),
		VisibleChunks: len(g.visibleChunks),
		MinDistance:   float32(math.MaxFloat32),
		MaxDistance:   0,
		UpdateTime:    time.Since(time.Unix(0, g.lastUpdate)),
	}

	// Count chunks by LOD level and status
	maxLod := 0
	totalDistance := float32(0)
	chunksByStatus := make(map[ChunkStatus]int)

	for _, chunk := range g.chunks {
		// Track LOD levels
		if chunk.LODLevel > maxLod {
			maxLod = chunk.LODLevel
		}

		// Track distances
		if chunk.Distance > stats.MaxDistance {
			stats.MaxDistance = chunk.Distance
		}
		if chunk.Distance < stats.MinDistance && chunk.Distance > 0 {
			stats.MinDistance = chunk.Distance
		}
		totalDistance += chunk.Distance

		// Track statuses
		chunksByStatus[chunk.Status]++
	}

	// Initialize LOD counts array
	stats.ChunksByLOD = make([]int, maxLod+1)

	// Fill LOD counts
	for _, chunk := range g.chunks {
		stats.ChunksByLOD[chunk.LODLevel]++
	}

	// Get counts by status
	stats.LoadedChunks = chunksByStatus[ChunkLoaded]
	stats.PendingChunks = chunksByStatus[ChunkLoading]

	// Calculate average distance
	if len(g.chunks) > 0 {
		stats.AverageDistance = totalDistance / float32(len(g.chunks))
	}

	// Count neighbors and find the bounds
	totalNeighbors := 0
	maxNeighbors := 0

	minX, maxX := math.MaxInt32, math.MinInt32
	minZ, maxZ := math.MaxInt32, math.MinInt32

	for _, chunk := range g.chunks {
		// Count neighbors
		neighborCount := 0
		for _, neighbor := range chunk.Neighbors {
			if neighbor != nil {
				neighborCount++
			}
		}

		totalNeighbors += neighborCount
		if neighborCount > maxNeighbors {
			maxNeighbors = neighborCount
		}

		// Track grid bounds
		if chunk.GridCoord.X < minX {
			minX = chunk.GridCoord.X
		}
		if chunk.GridCoord.X > maxX {
			maxX = chunk.GridCoord.X
		}
		if chunk.GridCoord.Z < minZ {
			minZ = chunk.GridCoord.Z
		}
		if chunk.GridCoord.Z > maxZ {
			maxZ = chunk.GridCoord.Z
		}
	}

	stats.MaxNeighbors = maxNeighbors
	if len(g.chunks) > 0 {
		stats.AverageNeighbors = float32(totalNeighbors) / float32(len(g.chunks))
	}

	stats.GridBounds = GridBounds{
		MinX: minX, MaxX: maxX,
		MinZ: minZ, MaxZ: maxZ,
	}

	return stats
}

// GetDetailedStats returns a detailed string representation of grid statistics
func (g *TerrainGrid) GetDetailedStats() string {
	stats := g.Stats()

	var sb strings.Builder

	sb.WriteString("Terrain Grid System Statistics:\n")
	sb.WriteString("==============================\n\n")

	// General information
	sb.WriteString(fmt.Sprintf("Total Chunks: %d\n", stats.TotalChunks))
	sb.WriteString(fmt.Sprintf("Visible Chunks: %d (%.1f%%)\n",
		stats.VisibleChunks,
		percentOrZero(stats.VisibleChunks, stats.TotalChunks)))
	sb.WriteString(fmt.Sprintf("Loaded Chunks: %d (%.1f%%)\n",
		stats.LoadedChunks,
		percentOrZero(stats.LoadedChunks, stats.TotalChunks)))
	sb.WriteString(fmt.Sprintf("Pending Chunks: %d (%.1f%%)\n",
		stats.PendingChunks,
		percentOrZero(stats.PendingChunks, stats.TotalChunks)))
	sb.WriteString(fmt.Sprintf("Last Update: %v ago\n\n", stats.UpdateTime))

	// LOD breakdown
	sb.WriteString("LOD Distribution:\n")
	for i, count := range stats.ChunksByLOD {
		sb.WriteString(fmt.Sprintf("  LOD %d: %d chunks (%.1f%%)\n",
			i, count, percentOrZero(count, stats.TotalChunks)))
	}
	sb.WriteString("\n")

	// Distance information
	sb.WriteString("Distance Stats:\n")
	sb.WriteString(fmt.Sprintf("  Min Distance: %.1f\n", stats.MinDistance))
	sb.WriteString(fmt.Sprintf("  Max Distance: %.1f\n", stats.MaxDistance))
	sb.WriteString(fmt.Sprintf("  Average Distance: %.1f\n\n", stats.AverageDistance))

	// Neighbor information
	sb.WriteString("Neighbor Stats:\n")
	sb.WriteString(fmt.Sprintf("  Average Neighbors: %.1f\n", stats.AverageNeighbors))
	sb.WriteString(fmt.Sprintf("  Max Neighbors: %d\n\n", stats.MaxNeighbors))

	// Grid bounds
	sb.WriteString("Grid Bounds:\n")
	sb.WriteString(fmt.Sprintf("  X: %d to %d (width: %d)\n",
		stats.GridBounds.MinX, stats.GridBounds.MaxX,
		stats.GridBounds.MaxX-stats.GridBounds.MinX+1))
	sb.WriteString(fmt.Sprintf("  Z: %d to %d (depth: %d)\n",
		stats.GridBounds.MinZ, stats.GridBounds.MaxZ,
		stats.GridBounds.MaxZ-stats.GridBounds.MinZ+1))

	return sb.String()
}

// VisualizeGrid generates a 2D top-down visualization of the grid
func (g *TerrainGrid) VisualizeGrid() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Grid Visualization (Top-Down View):\n")
	sb.WriteString("=================================\n\n")

	// Get the camera position's grid cell
	cameraGridCoord := g.WorldToGrid(g.lastCameraPos)

	// Find grid bounds for display
	minX, maxX := math.MaxInt32, math.MinInt32
	minZ, maxZ := math.MaxInt32, math.MinInt32

	for coord := range g.chunksByCoord {
		if coord.X < minX {
			minX = coord.X
		}
		if coord.X > maxX {
			maxX = coord.X
		}
		if coord.Z < minZ {
			minZ = coord.Z
		}
		if coord.Z > maxZ {
			maxZ = coord.Z
		}
	}

	// Expand bounds to include camera position
	if cameraGridCoord.X < minX {
		minX = cameraGridCoord.X
	}
	if cameraGridCoord.X > maxX {
		maxX = cameraGridCoord.X
	}
	if cameraGridCoord.Z < minZ {
		minZ = cameraGridCoord.Z
	}
	if cameraGridCoord.Z > maxZ {
		maxZ = cameraGridCoord.Z
	}

	// Constrain size for display
	const maxDisplayWidth = 40
	const maxDisplayHeight = 20

	width := maxX - minX + 1
	height := maxZ - minZ + 1

	if width > maxDisplayWidth {
		centerX := (minX + maxX) / 2
		minX = centerX - maxDisplayWidth/2
		maxX = centerX + maxDisplayWidth/2
	}
	if height > maxDisplayHeight {
		centerZ := (minZ + maxZ) / 2
		minZ = centerZ - maxDisplayHeight/2
		maxZ = centerZ + maxDisplayHeight/2
	}

	// Print column headers (x coordinates)
	sb.WriteString("     ") // Space for row headers
	for x := minX; x <= maxX; x++ {
		if x%5 == 0 {
			sb.WriteString(fmt.Sprintf("%-5d", x))
		} else {
			sb.WriteString("     ")
		}
	}
	sb.WriteString("\n")

	// Print horizontal line
	sb.WriteString("     +")
	for x := minX; x <= maxX; x++ {
		sb.WriteString("-----")
	}
	sb.WriteString("+\n")

	// Print rows
	for z := minZ; z <= maxZ; z++ {
		// Row header (z coordinate)
		if z%5 == 0 {
			sb.WriteString(fmt.Sprintf("%4d |", z))
		} else {
			sb.WriteString("     |")
		}

		// Print grid cells
		for x := minX; x <= maxX; x++ {
			cell := GridCoord{X: x, Z: z}
			chunk := g.chunksByCoord[cell]

			// Determine what symbol to display
			var symbol string

			if x == cameraGridCoord.X && z == cameraGridCoord.Z {
				// Camera position
				symbol = "  C  "
			} else if chunk == nil {
				// Empty cell
				symbol = "  .  "
			} else {
				// Base symbol based on chunk status
				var baseSymbol string
				switch chunk.Status {
				case ChunkLoaded:
					if chunk.LODLevel <= 9 {
						baseSymbol = fmt.Sprintf("  %d  ", chunk.LODLevel)
					} else {
						baseSymbol = "  +  "
					}
				case ChunkLoading:
					baseSymbol = "  L  "
				case ChunkUnloaded:
					baseSymbol = "  U  "
				case ChunkError:
					baseSymbol = "  E  "
				default:
					baseSymbol = "  ?  "
				}

				// Modify display for in-frustum chunks
				if chunk.InFrustum {
					// For in-frustum chunks, we'll use * as visual indicator
					symbol = fmt.Sprintf("*%s*", baseSymbol[1:4])
				} else {
					symbol = baseSymbol
				}
			}

			sb.WriteString(symbol)
		}

		sb.WriteString("|\n")
	}

	// Print horizontal line
	sb.WriteString("     +")
	for x := minX; x <= maxX; x++ {
		sb.WriteString("-----")
	}
	sb.WriteString("+\n\n")

	// Add legend
	sb.WriteString("Legend:\n")
	sb.WriteString("  C = Camera Position\n")
	sb.WriteString("  . = Empty Cell\n")
	sb.WriteString("  0-9 = Loaded Chunk (LOD Level)\n")
	sb.WriteString("  + = Loaded Chunk (LOD > 9)\n")
	sb.WriteString("  L = Loading Chunk\n")
	sb.WriteString("  U = Unloaded Chunk\n")
	sb.WriteString("  E = Error Loading Chunk\n")
	sb.WriteString("  * = In View Frustum (surrounds symbol)\n")

	return sb.String()
}

// VisualizeChunk generates a detailed visualization of a specific chunk and its neighbors
func (g *TerrainGrid) VisualizeChunk(chunkID string) string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	chunk, exists := g.chunks[chunkID]
	if !exists {
		return fmt.Sprintf("Chunk %s not found", chunkID)
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Chunk Visualization: %s\n", chunkID))
	sb.WriteString("==================================\n\n")

	// Basic info
	sb.WriteString(fmt.Sprintf("Grid Coordinates: %v\n", chunk.GridCoord))
	sb.WriteString(fmt.Sprintf("World Position: (%.1f, %.1f)\n",
		chunk.WorldPos[0], chunk.WorldPos[1]))
	sb.WriteString(fmt.Sprintf("LOD Level: %d\n", chunk.LODLevel))
	sb.WriteString(fmt.Sprintf("Status: %s\n", chunkStatusString(chunk.Status)))
	sb.WriteString(fmt.Sprintf("Distance from Camera: %.1f\n", chunk.Distance))
	sb.WriteString(fmt.Sprintf("In Frustum: %v\n", chunk.InFrustum))
	sb.WriteString(fmt.Sprintf("Priority: %.1f\n\n", chunk.Priority))

	// Neighbor visualization
	sb.WriteString("Neighbors:\n")
	sb.WriteString("    ┌───┐    \n")
	sb.WriteString(fmt.Sprintf("    │ %s │    \n", neighborSymbol(chunk.Neighbors[North])))
	sb.WriteString("┌───┼───┼───┐\n")
	sb.WriteString(fmt.Sprintf("│ %s │ C │ %s │\n",
		neighborSymbol(chunk.Neighbors[West]),
		neighborSymbol(chunk.Neighbors[East])))
	sb.WriteString("└───┼───┼───┘\n")
	sb.WriteString(fmt.Sprintf("    │ %s │    \n", neighborSymbol(chunk.Neighbors[South])))
	sb.WriteString("    └───┘    \n\n")

	// LOD transition info
	sb.WriteString("LOD Transitions:\n")
	hasLODMismatch := false
	for dir, neighbor := range chunk.Neighbors {
		if neighbor != nil && neighbor.LODLevel != chunk.LODLevel {
			hasLODMismatch = true
			sb.WriteString(fmt.Sprintf("  %s: LOD %d -> LOD %d (Δ%d)\n",
				DirectionNames[Direction(dir)],
				chunk.LODLevel,
				neighbor.LODLevel,
				neighbor.LODLevel-chunk.LODLevel))
		}
	}
	if !hasLODMismatch {
		sb.WriteString("  None - all neighbors at same LOD level\n")
	}

	return sb.String()
}

// VisualizeVisibility generates a visualization of chunk visibility and LOD levels
func (g *TerrainGrid) VisualizeVisibility() string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Visibility and LOD Distribution:\n")
	sb.WriteString("==============================\n\n")

	// Group chunks by distance for visualization
	type distanceGroup struct {
		minDist, maxDist float32
		chunks           []*ChunkInfo
	}

	groups := []distanceGroup{
		{0, 100, []*ChunkInfo{}},
		{100, 200, []*ChunkInfo{}},
		{200, 400, []*ChunkInfo{}},
		{400, 800, []*ChunkInfo{}},
		{800, 1600, []*ChunkInfo{}},
		{1600, float32(math.MaxFloat32), []*ChunkInfo{}},
	}

	// Categorize chunks by distance
	for _, chunk := range g.chunks {
		for i := range groups {
			if chunk.Distance >= groups[i].minDist && chunk.Distance < groups[i].maxDist {
				groups[i].chunks = append(groups[i].chunks, chunk)
				break
			}
		}
	}

	// Display chunk distribution by distance and LOD
	for _, group := range groups {
		if len(group.chunks) == 0 {
			continue
		}

		// Count chunks by LOD and visibility
		lodCounts := make(map[int]int)
		lodVisible := make(map[int]int)

		for _, chunk := range group.chunks {
			lodCounts[chunk.LODLevel]++
			if chunk.InFrustum {
				lodVisible[chunk.LODLevel]++
			}
		}

		// Sort LOD levels
		lods := make([]int, 0, len(lodCounts))
		for lod := range lodCounts {
			lods = append(lods, lod)
		}
		sort.Ints(lods)

		// Display headers
		var rangeStr string
		if group.maxDist == float32(math.MaxFloat32) {
			rangeStr = fmt.Sprintf("%.0f+", group.minDist)
		} else {
			rangeStr = fmt.Sprintf("%.0f-%.0f", group.minDist, group.maxDist)
		}

		sb.WriteString(fmt.Sprintf("Distance %s: %d chunks\n", rangeStr, len(group.chunks)))

		// Display LOD distribution
		for _, lod := range lods {
			visible := lodVisible[lod]
			total := lodCounts[lod]

			// Create a bar graph
			const maxBarWidth = 40
			barWidth := int(float64(total) / float64(len(group.chunks)) * maxBarWidth)
			visibleWidth := int(float64(visible) / float64(len(group.chunks)) * maxBarWidth)

			bar := strings.Repeat("■", visibleWidth) +
				strings.Repeat("□", barWidth-visibleWidth)

			sb.WriteString(fmt.Sprintf("  LOD %d: %s (%d total, %d visible)\n",
				lod, bar, total, visible))
		}
		sb.WriteString("\n")
	}

	// Add legend
	sb.WriteString("Legend:\n")
	sb.WriteString("  ■ = Visible chunks\n")
	sb.WriteString("  □ = Non-visible chunks\n")

	return sb.String()
}

// Helper functions

// percentOrZero calculates the percentage or returns 0 if divisor is 0
func percentOrZero(num, total int) float32 {
	if total == 0 {
		return 0
	}
	return float32(num) / float32(total) * 100
}

// getHighlightedSymbol returns a symbol that stands out (for in-frustum chunks)
func getHighlightedSymbol(symbol string) string {
	return fmt.Sprintf("[%s]", symbol)
}

// neighborSymbol returns a symbol representing the neighbor's LOD
func neighborSymbol(chunk *ChunkInfo) string {
	if chunk == nil {
		return " "
	}

	if chunk.LODLevel <= 9 {
		return fmt.Sprintf("%d", chunk.LODLevel)
	}
	return "+"
}

// chunkStatusString returns a human-readable status string
func chunkStatusString(status ChunkStatus) string {
	switch status {
	case ChunkUnloaded:
		return "Unloaded"
	case ChunkLoading:
		return "Loading"
	case ChunkLoaded:
		return "Loaded"
	case ChunkError:
		return "Error"
	default:
		return "Unknown"
	}
}
