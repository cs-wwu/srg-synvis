package memory

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
	"unsafe"

	vk "github.com/vulkan-go/vulkan"
)

// BucketAllocator implements MemoryAllocator interface using a pool-based approach
// for fixed-size terrain chunks, allowing memory reuse and efficient allocation.
type BucketAllocator struct {
	totalMemory vk.DeviceSize
	buckets     []MemoryBucket
	device      *vk.Device
	memProps    *vk.PhysicalDeviceMemoryProperties

	// Thread safety
	mu sync.RWMutex
}

// MemoryBucket manages a fixed-size pool of memory for a specific LOD level.
type MemoryBucket struct {
	buffer     vk.Buffer       // Vulkan buffer for this bucket
	memory     vk.DeviceMemory // Vulkan device memory
	percentage float32         // Percentage of total memory allocated to this bucket
	size       vk.DeviceSize   // Total size of the bucket
	chunkSize  vk.DeviceSize   // Size of each chunk in this bucket
	slotCount  int             // Total number of slots in this bucket

	slotStack   *SlotStack     // Tracks available slots
	bitset      *Bitset        // Compact representation of slot usage
	allocations map[string]int // Maps chunkID to slotIndex

	// Thread safety
	mu sync.Mutex
}

// NewBucketAllocator creates a new fixed-size chunk pool allocator.
func NewBucketAllocator(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties, percentages []float32, chunkSizes []vk.DeviceSize) *BucketAllocator {
	// Validate input
	now := time.Now()
	if len(percentages) == 0 || len(percentages) != len(chunkSizes) {
		panic("percentages and chunkSizes must be non-empty and of equal length")
	}
	var sum float32
	for _, p := range percentages {
		sum += p
	}
	if sum < 0.99 || sum > 1.01 {
		panic("percentages must sum to 1.0")
	}

	// Find largest device-local heap
	totalMemory := vk.DeviceSize(0)
	for i := uint32(0); i < memProps.MemoryHeapCount; i++ {
		memProps.MemoryHeaps[i].Deref()
		heap := memProps.MemoryHeaps[i]
		if heap.Flags&vk.MemoryHeapFlags(vk.MemoryHeapDeviceLocalBit) != 0 && heap.Size > totalMemory {
			totalMemory = heap.Size
		}
	}
	if totalMemory == 0 {
		panic("No device-local heap found")
	}
	budget := vk.DeviceSize(float64(totalMemory) * 0.7) // 70% safety margin
	fmt.Printf("Total device-local memory: %.1fMB\n", float32(totalMemory)/(1024*1024))
	fmt.Printf("Allocated memory budget: %.1fMB\n", float32(budget)/(1024*1024))

	// Create buckets
	buckets := make([]MemoryBucket, len(percentages))
	for i, perc := range percentages {
		bucketSize := vk.DeviceSize(float64(budget) * float64(perc))
		slotCount := int(bucketSize / chunkSizes[i])

		// Create buffer and allocate memory
		internalNow := time.Now()
		buffer, memory := createBufferAndMemory(device, memProps, bucketSize)
		fmt.Printf(" bucket %d (%.2f%%) took: %s\n", i, perc, time.Since(internalNow))

		// Initialize slot stack and bitset
		freeList := NewFreeList(slotCount)
		bitset := NewBitset(slotCount)

		buckets[i] = MemoryBucket{
			buffer:      buffer,
			memory:      memory,
			percentage:  perc,
			size:        bucketSize,
			chunkSize:   chunkSizes[i],
			slotCount:   slotCount,
			slotStack:   freeList,
			bitset:      bitset,
			allocations: make(map[string]int),
		}
	}

	fmt.Printf("Bucket allocator initialization took: %s\n", time.Since(now))
	fmt.Printf("Total allocated memory: %.1fMB\n", float32(budget)/(1024*1024))
	fmt.Printf("Total number of slots: %d\n", len(buckets)*len(chunkSizes))

	return &BucketAllocator{
		totalMemory: budget,
		buckets:     buckets,
		device:      device,
		memProps:    memProps,
	}
}

// AllocateChunk allocates space for a chunk in the specified LOD bucket.
// Returns (success, offset in buffer).
func (a *BucketAllocator) AllocateChunk(chunkID string, chunkSize vk.DeviceSize, lodLevel int) (bool, vk.DeviceSize) {
	// Lock bucket for allocation
	a.mu.RLock()
	if lodLevel < 0 || lodLevel >= len(a.buckets) {
		a.mu.RUnlock()
		return false, 0
	}
	bucket := &a.buckets[lodLevel]
	a.mu.RUnlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Check if the chunk size is appropriate for this bucket
	if chunkSize > bucket.chunkSize {
		log.Printf("Warning: chunkSize > bucket.chunkSize, may cause fragmentation")
	}

	// Check if there's a free slot
	slotIndex, ok := bucket.slotStack.Pop()
	if !ok {
		return false, 0 // Not enough space
	}

	// Mark slot as allocated
	bucket.bitset.Set(slotIndex, 1)

	// Calculate offset
	offset := vk.DeviceSize(slotIndex) * bucket.chunkSize

	// Track allocation
	bucket.allocations[chunkID] = slotIndex

	return true, offset
}

// FreeChunk releases a chunk allocation in a bucket (does not compact memory).
func (a *BucketAllocator) FreeChunk(chunkID string, lodLevel int) bool {
	// Lock bucket for deallocation
	a.mu.RLock()
	if lodLevel < 0 || lodLevel >= len(a.buckets) {
		a.mu.RUnlock()
		return false
	}
	bucket := &a.buckets[lodLevel]
	a.mu.RUnlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Verify chunk exists
	slotIndex, ok := bucket.allocations[chunkID]
	if !ok {
		return false
	}

	// Mark slot as free
	bucket.bitset.Set(slotIndex, 0)

	// Add slot back to slot stack
	bucket.slotStack.Push(slotIndex)

	// Remove from active allocations
	delete(bucket.allocations, chunkID)

	return true
}

// Destroy releases all Vulkan buffers and memory.
func (a *BucketAllocator) Destroy() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i := range a.buckets {
		bucket := &a.buckets[i]
		vk.FreeMemory(*a.device, bucket.memory, nil)
		vk.DestroyBuffer(*a.device, bucket.buffer, nil)
	}
}

// Stats returns memory usage statistics.
func (a *BucketAllocator) Stats() AllocatorStats {
	a.mu.RLock()
	defer a.mu.RUnlock()

	stats := AllocatorStats{}

	for i := range a.buckets {
		bucket := &a.buckets[i]
		bucket.mu.Lock()
		stats.TotalSize += uint32(bucket.size)
		allocatedSize := uint32(len(bucket.allocations)) * uint32(bucket.chunkSize)
		stats.UsedSize += allocatedSize
		stats.Allocations += len(bucket.allocations)
		bucket.mu.Unlock()
	}

	stats.FreeSize = stats.TotalSize - stats.UsedSize

	return stats
}

// Helper function to create buffer and allocate memory.
// TODO: this function takes ~120ms per bucket
// Refactor to using single buffer, and subdiving it into the buckets myself
func createBufferAndMemory(device *vk.Device, memProps *vk.PhysicalDeviceMemoryProperties, size vk.DeviceSize) (vk.Buffer, vk.DeviceMemory) {
	// Create buffer (no initial data, just reserve space)
	var buffer vk.Buffer
	ret := vk.CreateBuffer(*device, &vk.BufferCreateInfo{
		SType: vk.StructureTypeBufferCreateInfo,
		Size:  size,
		Usage: vk.BufferUsageFlags(vk.BufferUsageVertexBufferBit | vk.BufferUsageIndexBufferBit),
	}, nil, &buffer)
	if ret != vk.Success {
		panic(fmt.Sprintf("failed to create buffer: %d", ret))
	}

	// Get memory requirements
	var memReqs vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(*device, buffer, &memReqs)
	memReqs.Deref()

	// Find suitable memory type
	memTypeIndex := findMemoryType(memProps, uint32(memReqs.MemoryTypeBits), vk.MemoryPropertyDeviceLocalBit)

	// Allocate memory
	var memory vk.DeviceMemory
	ret = vk.AllocateMemory(*device, &vk.MemoryAllocateInfo{
		SType:           vk.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memReqs.Size,
		MemoryTypeIndex: memTypeIndex,
	}, nil, &memory)
	if ret != vk.Success {
		vk.DestroyBuffer(*device, buffer, nil)
		panic(fmt.Sprintf("failed to allocate memory: %d", ret))
	}

	// Bind buffer to memory
	ret = vk.BindBufferMemory(*device, buffer, memory, 0)
	if ret != vk.Success {
		vk.DestroyBuffer(*device, buffer, nil)
		vk.FreeMemory(*device, memory, nil)
		panic(fmt.Sprintf("failed to bind buffer memory: %d", ret))
	}

	return buffer, memory
}

// Helper function to find suitable memory type.
func findMemoryType(memProps *vk.PhysicalDeviceMemoryProperties, typeFilter uint32, properties vk.MemoryPropertyFlagBits) uint32 {
	for i := uint32(0); i < memProps.MemoryTypeCount; i++ {
		memProps.MemoryTypes[i].Deref()
		if (typeFilter&(1<<i)) != 0 && (memProps.MemoryTypes[i].PropertyFlags&vk.MemoryPropertyFlags(properties)) == vk.MemoryPropertyFlags(properties) {
			return i
		}
	}
	panic("failed to find suitable memory type")
}

// Upload uploads data to an allocated chunk.
func (a *BucketAllocator) Upload(chunkID string, lodLevel int, data []byte) error {
	a.mu.RLock()
	if lodLevel < 0 || lodLevel >= len(a.buckets) {
		a.mu.RUnlock()
		return fmt.Errorf("invalid LOD level: %d", lodLevel)
	}
	bucket := &a.buckets[lodLevel]
	a.mu.RUnlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Verify chunk exists
	slotIndex, ok := bucket.allocations[chunkID]
	if !ok {
		return fmt.Errorf("chunk not found: %s", chunkID)
	}

	// Calculate offset
	offset := vk.DeviceSize(slotIndex) * bucket.chunkSize

	// Map memory temporarily
	var pData unsafe.Pointer
	ret := vk.MapMemory(*a.device, bucket.memory, offset, vk.DeviceSize(len(data)), 0, &pData)
	if ret != vk.Success {
		return fmt.Errorf("failed to map memory: %d", ret)
	}

	// Copy data to GPU memory
	vk.Memcopy(pData, data)

	// Unmap memory
	vk.UnmapMemory(*a.device, bucket.memory)

	return nil
}

// GetAllocationInfo returns information about an allocated chunk.
func (a *BucketAllocator) GetAllocationInfo(chunkID string, lodLevel int) (AllocationInfo, error) {
	a.mu.RLock()
	if lodLevel < 0 || lodLevel >= len(a.buckets) {
		a.mu.RUnlock()
		return AllocationInfo{}, fmt.Errorf("invalid LOD level: %d", lodLevel)
	}
	bucket := &a.buckets[lodLevel]
	a.mu.RUnlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Verify chunk exists
	slotIndex, ok := bucket.allocations[chunkID]
	if !ok {
		return AllocationInfo{}, fmt.Errorf("chunk not found: %s", chunkID)
	}

	// Calculate offset
	offset := vk.DeviceSize(slotIndex) * bucket.chunkSize

	return AllocationInfo{
		Buffer: bucket.buffer,
		Offset: offset,
		Size:   bucket.chunkSize,
	}, nil
}

// GetDetailedStats returns detailed memory statistics.
func (a *BucketAllocator) GetDetailedStats() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Fixed-Size Chunk Pool Allocator Statistics:\n")
	sb.WriteString("==========================================\n\n")

	totalSize := vk.DeviceSize(0)
	totalUsed := vk.DeviceSize(0)

	for i := range a.buckets {
		bucket := &a.buckets[i]
		bucket.mu.Lock()

		totalSize += bucket.size
		usedSize := vk.DeviceSize(len(bucket.allocations)) * bucket.chunkSize
		totalUsed += usedSize

		sb.WriteString(fmt.Sprintf("Bucket %d (LOD %d):\n", i, i))
		sb.WriteString(fmt.Sprintf("  Total Size: %d bytes\n", bucket.size))
		sb.WriteString(fmt.Sprintf("  Chunk Size: %d bytes\n", bucket.chunkSize))
		sb.WriteString(fmt.Sprintf("  Total Slots: %d\n", bucket.slotCount))
		sb.WriteString(fmt.Sprintf("  Used Slots: %d (%.1f%%)\n",
			len(bucket.allocations),
			float32(len(bucket.allocations))/float32(bucket.slotCount)*100.0))
		sb.WriteString(fmt.Sprintf("  Free Slots: %d\n", bucket.slotCount-len(bucket.allocations)))
		sb.WriteString("\n")

		bucket.mu.Unlock()
	}

	sb.WriteString("Overall:\n")
	sb.WriteString(fmt.Sprintf("  Total Memory: %d bytes\n", totalSize))
	sb.WriteString(fmt.Sprintf("  Used Memory: %d bytes (%.1f%%)\n",
		totalUsed,
		float32(totalUsed)/float32(totalSize)*100.0))
	sb.WriteString(fmt.Sprintf("  Free Memory: %d bytes\n", totalSize-totalUsed))

	return sb.String()
}

// VisualizeMemoryMap generates a visual representation of memory allocation state.
func (a *BucketAllocator) VisualizeMemoryMap() string {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var sb strings.Builder

	sb.WriteString("Memory Map Visualization:\n")
	sb.WriteString("========================\n\n")

	for i := range a.buckets {
		bucket := &a.buckets[i]
		bucket.mu.Lock()

		sb.WriteString(fmt.Sprintf("Bucket %d (LOD %d, %d slots):\n", i, i, bucket.slotCount))
		sb.WriteString("[")

		// Display a sample of the memory allocation (up to 120 slots)
		displayLimit := min(bucket.slotCount, 120)

		for j := range displayLimit {
			if bucket.bitset.IsSet(j) {
				sb.WriteString("A")
			} else {
				sb.WriteString("F")
			}

			// Line break every 80 characters
			if (j+1)%80 == 0 && j+1 < displayLimit {
				sb.WriteString("]\n[")
			}
		}

		if displayLimit < bucket.slotCount {
			sb.WriteString("...")
		}

		sb.WriteString("]\n\n")

		bucket.mu.Unlock()
	}

	sb.WriteString("Legend: [A] = Allocated, [F] = Free\n")

	return sb.String()
}

// DefferedSlotStack tracks chunks to be deallocated in batch.
type DefferedSlotStack struct {
	chunks []struct {
		chunkID  string
		lodLevel int
	}
	mu sync.Mutex
}

// NewDefferedSlotStack creates a new deferred slot stack.
func NewDefferedSlotStack() *DefferedSlotStack {
	return &DefferedSlotStack{
		chunks: make([]struct {
			chunkID  string
			lodLevel int
		}, 0, 100), // Pre-allocate for 100 pending deallocations
	}
}

// Add adds a chunk to the deferred slot stack.
func (d *DefferedSlotStack) Add(chunkID string, lodLevel int) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.chunks = append(d.chunks, struct {
		chunkID  string
		lodLevel int
	}{chunkID, lodLevel})
}

// ProcessAll frees all chunks in the deferred slot stack.
func (d *DefferedSlotStack) ProcessAll(allocator *BucketAllocator) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	count := 0
	for _, chunk := range d.chunks {
		if allocator.FreeChunk(chunk.chunkID, chunk.lodLevel) {
			count++
		}
	}

	d.chunks = d.chunks[:0] // Clear the list
	return count
}

// Count returns the number of chunks in the deferred slot stack.
func (d *DefferedSlotStack) Count() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return len(d.chunks)
}
