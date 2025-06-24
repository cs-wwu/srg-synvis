package rendering

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

// PerformanceStats tracks detailed performance metrics
type PerformanceStats struct {
	// Timing
	FPS          float64
	FrameTimeMs  float64
	CPUTimeMs    float64
	RenderTimeMs float64

	// Rendering
	VertexCount   int
	TriangleCount int
	DrawCalls     int
	ChunkCount    int

	// Memory
	SystemMemoryMB float64
	HeapMemoryMB   float64
	GCPauses       []time.Duration

	// System
	ValidationLayers bool
	PresentMode      string
	FrameLag         int
}

// StatsTracker manages performance measurement and rolling statistics
type StatsTracker struct {
	enabled bool
	mu      sync.RWMutex

	// Current frame measurements
	frameStart    time.Time
	cpuStart      time.Time
	renderStart   time.Time
	lastFrameTime time.Time

	// Rolling statistics
	frameTimes  []float64
	cpuTimes    []float64
	renderTimes []float64

	// Frame counters
	frameCount int64
	fpsStart   time.Time
	currentFPS float64

	// Current stats
	current PerformanceStats

	// Configuration
	maxSamples   int
	reportPeriod time.Duration
	lastReport   time.Time
}

// NewStatsTracker creates a new performance tracker
func NewStatsTracker(maxSamples int, reportPeriod time.Duration) *StatsTracker {
	return &StatsTracker{
		enabled:      false,
		maxSamples:   maxSamples,
		reportPeriod: reportPeriod,
		frameTimes:   make([]float64, 0, maxSamples),
		cpuTimes:     make([]float64, 0, maxSamples),
		renderTimes:  make([]float64, 0, maxSamples),
		fpsStart:     time.Now(),
		lastReport:   time.Now(),
	}
}

// Enable starts performance tracking
func (st *StatsTracker) Enable() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.enabled = true
	st.fpsStart = time.Now()
	st.frameCount = 0
}

// Disable stops performance tracking
func (st *StatsTracker) Disable() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.enabled = false
}

// StartFrame begins timing a new frame
func (st *StatsTracker) StartFrame() {
	if !st.enabled {
		return
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	now := time.Now()
	st.frameStart = now
	st.cpuStart = now

	// Calculate frame time from previous frame
	if !st.lastFrameTime.IsZero() {
		frameTime := now.Sub(st.lastFrameTime).Seconds() * 1000 // ms
		st.addSample(&st.frameTimes, frameTime)
		st.current.FrameTimeMs = frameTime
	}
	st.lastFrameTime = now

	st.frameCount++
}

// StartRender marks the beginning of GPU rendering work
func (st *StatsTracker) StartRender() {
	if !st.enabled {
		return
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// CPU time is from frame start to render start
	cpuTime := time.Since(st.cpuStart).Seconds() * 1000 // ms
	st.addSample(&st.cpuTimes, cpuTime)
	st.current.CPUTimeMs = cpuTime

	st.renderStart = time.Now()
}

// EndRender marks the end of GPU rendering work
func (st *StatsTracker) EndRender() {
	if !st.enabled {
		return
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	renderTime := time.Since(st.renderStart).Seconds() * 1000 // ms
	st.addSample(&st.renderTimes, renderTime)
	st.current.RenderTimeMs = renderTime
}

// UpdateRenderingStats updates rendering-specific metrics
func (st *StatsTracker) UpdateRenderingStats(vertexCount, triangleCount, drawCalls, chunkCount int) {
	if !st.enabled {
		return
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	st.current.VertexCount = vertexCount
	st.current.TriangleCount = triangleCount
	st.current.DrawCalls = drawCalls
	st.current.ChunkCount = chunkCount
}

// UpdateSystemStats updates system-level metrics
func (st *StatsTracker) UpdateSystemStats(validationLayers bool, presentMode string, frameLag int) {
	if !st.enabled {
		return
	}

	st.mu.Lock()
	defer st.mu.Unlock()

	// Memory stats
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	st.current.SystemMemoryMB = float64(memStats.Sys) / 1024 / 1024
	st.current.HeapMemoryMB = float64(memStats.HeapAlloc) / 1024 / 1024

	// System config
	st.current.ValidationLayers = validationLayers
	st.current.PresentMode = presentMode
	st.current.FrameLag = frameLag

	// Calculate FPS
	if elapsed := time.Since(st.fpsStart).Seconds(); elapsed > 0 {
		st.currentFPS = float64(st.frameCount) / elapsed
		st.current.FPS = st.currentFPS
	}
}

// GetCurrentStats returns the current performance statistics
func (st *StatsTracker) GetCurrentStats() PerformanceStats {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.current
}

// GetRollingStats returns statistical analysis of recent performance
func (st *StatsTracker) GetRollingStats() RollingStats {
	st.mu.RLock()
	defer st.mu.RUnlock()

	return RollingStats{
		FrameTime:   st.calculateStats(st.frameTimes),
		CPUTime:     st.calculateStats(st.cpuTimes),
		RenderTime:  st.calculateStats(st.renderTimes),
		SampleCount: len(st.frameTimes),
		DurationSec: time.Since(st.fpsStart).Seconds(),
	}
}

// ShouldReport returns true if it's time to print a stats report
func (st *StatsTracker) ShouldReport() bool {
	if !st.enabled {
		return false
	}
	return time.Since(st.lastReport) >= st.reportPeriod
}

// Report prints current performance statistics and resets the report timer
func (st *StatsTracker) Report() {
	if !st.enabled {
		return
	}

	st.mu.Lock()
	st.lastReport = time.Now()
	st.mu.Unlock()

	current := st.GetCurrentStats()
	rolling := st.GetRollingStats()

	fmt.Printf("\n=== Performance Report ===\n")
	fmt.Printf("FPS: %.1f | Frame: %.2fms | CPU: %.2fms | Render: %.2fms\n",
		current.FPS, current.FrameTimeMs, current.CPUTimeMs, current.RenderTimeMs)
	fmt.Printf("Vertices: %dk | Triangles: %dk | Draw Calls: %d | Chunks: %d\n",
		current.VertexCount/1000, current.TriangleCount/1000, current.DrawCalls, current.ChunkCount)
	fmt.Printf("Memory: System %.1fMB | Heap %.1fMB\n",
		current.SystemMemoryMB, current.HeapMemoryMB)
	fmt.Printf("Config: %s | Validation: %v | FrameLag: %d\n",
		current.PresentMode, current.ValidationLayers, current.FrameLag)

	if rolling.SampleCount > 10 {
		fmt.Printf("\nRolling Stats (%d samples, %.1fs):\n", rolling.SampleCount, rolling.DurationSec)
		fmt.Printf("Frame Time - Avg: %.2fms | P95: %.2fms | P99: %.2fms | Min: %.2fms | Max: %.2fms\n",
			rolling.FrameTime.Average, rolling.FrameTime.P95, rolling.FrameTime.P99,
			rolling.FrameTime.Min, rolling.FrameTime.Max)
	}
	fmt.Printf("=========================\n\n")
}

// Helper types for statistical analysis
type StatSummary struct {
	Average float64
	Min     float64
	Max     float64
	P95     float64
	P99     float64
	StdDev  float64
}

type RollingStats struct {
	FrameTime   StatSummary
	CPUTime     StatSummary
	RenderTime  StatSummary
	SampleCount int
	DurationSec float64
}

// Helper methods
func (st *StatsTracker) addSample(samples *[]float64, value float64) {
	*samples = append(*samples, value)
	if len(*samples) > st.maxSamples {
		*samples = (*samples)[1:] // Remove oldest sample
	}
}

func (st *StatsTracker) calculateStats(samples []float64) StatSummary {
	if len(samples) == 0 {
		return StatSummary{}
	}

	sorted := make([]float64, len(samples))
	copy(sorted, samples)
	sort.Float64s(sorted)

	// Basic stats
	sum := 0.0
	for _, v := range sorted {
		sum += v
	}
	avg := sum / float64(len(sorted))

	// Percentiles
	p95Idx := int(float64(len(sorted)) * 0.95)
	p99Idx := int(float64(len(sorted)) * 0.99)
	if p95Idx >= len(sorted) {
		p95Idx = len(sorted) - 1
	}
	if p99Idx >= len(sorted) {
		p99Idx = len(sorted) - 1
	}

	// Standard deviation
	sumSq := 0.0
	for _, v := range sorted {
		diff := v - avg
		sumSq += diff * diff
	}
	stdDev := 0.0
	if len(sorted) > 1 {
		stdDev = sumSq / float64(len(sorted)-1)
	}

	return StatSummary{
		Average: avg,
		Min:     sorted[0],
		Max:     sorted[len(sorted)-1],
		P95:     sorted[p95Idx],
		P99:     sorted[p99Idx],
		StdDev:  stdDev,
	}
}
