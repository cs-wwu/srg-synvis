package spatial

import (
	"fmt"
	"strings"
)

// GetDetailedStats returns a detailed string representation of chunk selection statistics
func (cs *ChunkSelector) GetDetailedStats() string {
	stats := cs.Stats()

	var sb strings.Builder

	sb.WriteString("Chunk Selection System Statistics:\n")
	sb.WriteString("=================================\n\n")

	// General information
	sb.WriteString(fmt.Sprintf("Desired Chunks: %d\n", stats.DesiredChunks))
	sb.WriteString(fmt.Sprintf("Available Chunks: %d\n", stats.AvailableChunks))
	sb.WriteString(fmt.Sprintf("Missing Chunks: %d\n", stats.MissingChunks))
	sb.WriteString(fmt.Sprintf("Camera Position: (%.1f, %.1f)\n", stats.LastCameraPos[0], stats.LastCameraPos[1]))
	sb.WriteString(fmt.Sprintf("Render Distance: %.1f\n\n", stats.LastRenderDist))

	// LOD breakdown
	cs.mu.RLock()
	lodCounts := make(map[int]int)
	lodMissing := make(map[int]int)

	for _, spec := range cs.desiredChunks {
		lodCounts[spec.LODLevel]++
		if !cs.availableChunks[spec.GridCoord] {
			lodMissing[spec.LODLevel]++
		}
	}
	cs.mu.RUnlock()

	sb.WriteString("LOD Distribution:\n")
	maxLOD := 0
	for lod := range lodCounts {
		if lod > maxLOD {
			maxLOD = lod
		}
	}

	for lod := 0; lod <= maxLOD; lod++ {
		total := lodCounts[lod]
		missing := lodMissing[lod]
		available := total - missing

		if total > 0 {
			sb.WriteString(fmt.Sprintf("  LOD %d: %d total (%d available, %d missing) - %.1f%% loaded\n",
				lod, total, available, missing,
				float32(available)/float32(total)*100.0))
		}
	}
	sb.WriteString("\n")

	// Frustum breakdown
	cs.mu.RLock()
	inFrustumCount := 0
	inFrustumMissing := 0
	outFrustumCount := 0
	outFrustumMissing := 0

	for _, spec := range cs.desiredChunks {
		if spec.InFrustum {
			inFrustumCount++
			if !cs.availableChunks[spec.GridCoord] {
				inFrustumMissing++
			}
		} else {
			outFrustumCount++
			if !cs.availableChunks[spec.GridCoord] {
				outFrustumMissing++
			}
		}
	}
	cs.mu.RUnlock()

	sb.WriteString("Frustum Distribution:\n")
	if inFrustumCount > 0 {
		sb.WriteString(fmt.Sprintf("  In Frustum: %d total (%d available, %d missing) - %.1f%% loaded\n",
			inFrustumCount, inFrustumCount-inFrustumMissing, inFrustumMissing,
			float32(inFrustumCount-inFrustumMissing)/float32(inFrustumCount)*100.0))
	}
	if outFrustumCount > 0 {
		sb.WriteString(fmt.Sprintf("  Out of Frustum: %d total (%d available, %d missing) - %.1f%% loaded\n",
			outFrustumCount, outFrustumCount-outFrustumMissing, outFrustumMissing,
			float32(outFrustumCount-outFrustumMissing)/float32(outFrustumCount)*100.0))
	}

	return sb.String()
}

// VisualizeSelection generates a 2D top-down visualization of the chunk selection
func (cs *ChunkSelector) VisualizeSelection() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Chunk Selection Visualization (Top-Down View):\n")
	sb.WriteString("============================================\n\n")

	if len(cs.desiredChunks) == 0 {
		sb.WriteString("No chunks selected\n")
		return sb.String()
	}

	// Get the camera position's grid cell
	cameraGridCoord := cs.WorldToGrid(cs.lastCameraPos)

	// Find bounds of desired chunks
	minX, maxX := cameraGridCoord.X, cameraGridCoord.X
	minZ, maxZ := cameraGridCoord.Z, cameraGridCoord.Z

	for coord := range cs.desiredChunks {
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
			coord := GridCoord{X: x, Z: z}

			var symbol string
			if x == cameraGridCoord.X && z == cameraGridCoord.Z {
				// Camera position
				symbol = "  C  "
			} else if spec, desired := cs.desiredChunks[coord]; desired {
				// Chunk is desired
				available := cs.availableChunks[coord]

				var baseSymbol string
				if available {
					// Available chunk - show LOD level
					if spec.LODLevel <= 9 {
						baseSymbol = fmt.Sprintf("  %d  ", spec.LODLevel)
					} else {
						baseSymbol = "  +  "
					}
				} else {
					// Missing chunk
					baseSymbol = "  M  "
				}

				// Modify display for in-frustum chunks
				if spec.InFrustum {
					symbol = fmt.Sprintf("*%s*", baseSymbol[1:4])
				} else {
					symbol = baseSymbol
				}
			} else {
				// Not desired
				symbol = "  .  "
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
	sb.WriteString("  . = Not Desired\n")
	sb.WriteString("  0-9 = Available Chunk (LOD Level)\n")
	sb.WriteString("  + = Available Chunk (LOD > 9)\n")
	sb.WriteString("  M = Missing Chunk (desired but not available)\n")
	sb.WriteString("  * = In View Frustum (surrounds symbol)\n")

	return sb.String()
}

// VisualizePriorities generates a visualization of chunk priorities
func (cs *ChunkSelector) VisualizePriorities() string {
	missing := cs.GetMissingChunks()

	var sb strings.Builder

	sb.WriteString("Missing Chunk Priorities:\n")
	sb.WriteString("========================\n\n")

	if len(missing) == 0 {
		sb.WriteString("No missing chunks\n")
		return sb.String()
	}

	// Show top 10 missing chunks by priority
	displayCount := min(len(missing), 10)

	sb.WriteString("Top Priority Missing Chunks:\n")
	for i := range displayCount {
		spec := missing[i]
		sb.WriteString(fmt.Sprintf("%2d. %8s - LOD:%d Dist:%.1f Priority:%.1f InFrustum:%v\n",
			i+1, spec.GridCoord.String(), spec.LODLevel, spec.Distance,
			spec.Priority, spec.InFrustum))
	}

	if len(missing) > displayCount {
		sb.WriteString(fmt.Sprintf("... and %d more\n", len(missing)-displayCount))
	}

	return sb.String()
}

// VisualizeLoadingEfficiency shows loading efficiency metrics
func (cs *ChunkSelector) VisualizeLoadingEfficiency() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Loading Efficiency Analysis:\n")
	sb.WriteString("===========================\n\n")

	// Distance-based analysis
	type distanceGroup struct {
		minDist, maxDist float32
		total, available int
	}

	groups := []distanceGroup{
		{0, 100, 0, 0},
		{100, 300, 0, 0},
		{300, 600, 0, 0},
		{600, 1000, 0, 0},
		{1000, 2000, 0, 0},
		{2000, 99999, 0, 0},
	}

	// Categorize chunks by distance
	for _, spec := range cs.desiredChunks {
		available := cs.availableChunks[spec.GridCoord]

		for i := range groups {
			if spec.Distance >= groups[i].minDist && spec.Distance < groups[i].maxDist {
				groups[i].total++
				if available {
					groups[i].available++
				}
				break
			}
		}
	}

	// Display efficiency by distance
	sb.WriteString("Efficiency by Distance:\n")
	for _, group := range groups {
		if group.total == 0 {
			continue
		}

		var rangeStr string
		if group.maxDist >= 99999 {
			rangeStr = fmt.Sprintf("%.0f+", group.minDist)
		} else {
			rangeStr = fmt.Sprintf("%.0f-%.0f", group.minDist, group.maxDist)
		}

		efficiency := float32(group.available) / float32(group.total) * 100.0

		// Create a simple bar graph
		const maxBarWidth = 30
		barWidth := int(efficiency / 100.0 * maxBarWidth)
		bar := strings.Repeat("■", barWidth) + strings.Repeat("□", maxBarWidth-barWidth)

		sb.WriteString(fmt.Sprintf("  %s: %s %.1f%% (%d/%d)\n",
			rangeStr, bar, efficiency, group.available, group.total))
	}

	return sb.String()
}

// AnalyzeNeighborCoverage analyzes how well chunks have loaded neighbors
func (cs *ChunkSelector) AnalyzeNeighborCoverage() string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Neighbor Coverage Analysis:\n")
	sb.WriteString("===========================\n\n")

	type neighborStats struct {
		chunkCount         int
		totalNeighbors     int
		availableNeighbors int
	}

	stats := neighborStats{}

	for coord := range cs.desiredChunks {
		if !cs.availableChunks[coord] {
			continue // Only analyze available chunks
		}

		stats.chunkCount++
		neighbors := cs.GetNeighborCoords(coord)

		for _, neighborCoord := range neighbors {
			if _, desired := cs.desiredChunks[neighborCoord]; desired {
				stats.totalNeighbors++
				if cs.availableChunks[neighborCoord] {
					stats.availableNeighbors++
				}
			}
		}
	}

	if stats.chunkCount == 0 {
		sb.WriteString("No available chunks to analyze\n")
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("Available Chunks: %d\n", stats.chunkCount))
	sb.WriteString(fmt.Sprintf("Total Desired Neighbors: %d\n", stats.totalNeighbors))
	sb.WriteString(fmt.Sprintf("Available Neighbors: %d\n", stats.availableNeighbors))

	if stats.totalNeighbors > 0 {
		efficiency := float32(stats.availableNeighbors) / float32(stats.totalNeighbors) * 100.0
		sb.WriteString(fmt.Sprintf("Neighbor Coverage: %.1f%%\n", efficiency))

		avgNeighbors := float32(stats.totalNeighbors) / float32(stats.chunkCount)
		avgAvailableNeighbors := float32(stats.availableNeighbors) / float32(stats.chunkCount)
		sb.WriteString(fmt.Sprintf("Average Neighbors per Chunk: %.1f (%.1f available)\n",
			avgNeighbors, avgAvailableNeighbors))
	}

	return sb.String()
}
