package memory

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type MathChunkPoolAllocator struct {
	buckets []mathPoolBucket
	mu      sync.Mutex // Add mutex for thread safety
}
type mathPoolBucket struct {
	slotCount   int
	chunkSize   uint64
	freeList    *SlotStack
	bitset      *Bitset
	allocations map[string]int
}

func NewMathChunkPoolAllocator(slotCounts []int, chunkSizes []uint64) *MathChunkPoolAllocator {
	buckets := make([]mathPoolBucket, len(slotCounts))
	for i := range slotCounts {
		buckets[i] = mathPoolBucket{
			slotCount:   slotCounts[i],
			chunkSize:   chunkSizes[i],
			freeList:    NewFreeList(slotCounts[i]),
			bitset:      NewBitset(slotCounts[i]),
			allocations: make(map[string]int),
		}
	}
	return &MathChunkPoolAllocator{buckets: buckets}
}

func (a *MathChunkPoolAllocator) AllocateChunk(chunkID string, chunkSize uint64, lodLevel int) (bool, uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if lodLevel < 0 || lodLevel >= len(a.buckets) {
		return false, 0
	}
	bucket := &a.buckets[lodLevel]
	if chunkSize > bucket.chunkSize {
		return false, 0
	}
	slotIndex, ok := bucket.freeList.Pop()
	if !ok {
		return false, 0
	}
	bucket.bitset.Set(slotIndex, 1)
	bucket.allocations[chunkID] = slotIndex
	return true, uint64(slotIndex) * bucket.chunkSize
}

func (a *MathChunkPoolAllocator) FreeChunk(chunkID string, lodLevel int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if lodLevel < 0 || lodLevel >= len(a.buckets) {
		return false
	}
	bucket := &a.buckets[lodLevel]
	slotIndex, ok := bucket.allocations[chunkID]
	if !ok {
		return false
	}
	bucket.bitset.Set(slotIndex, 0)
	bucket.freeList.Push(slotIndex)
	delete(bucket.allocations, chunkID)
	return true
}

func TestMathChunkPoolAllocator(t *testing.T) {
	assert := assert.New(t)

	slotCounts := []int{5, 3}
	chunkSizes := []uint64{100, 50}
	alloc := NewMathChunkPoolAllocator(slotCounts, chunkSizes)

	// Allocate all slots in bucket 0
	for i := range slotCounts[0] {
		ok, offset := alloc.AllocateChunk("chunkA"+strconv.Itoa(i), 100, 0)
		assert.True(ok)
		assert.Equal(uint64(i)*100, offset)
	}
	// Next allocation should fail
	ok, _ := alloc.AllocateChunk("chunkZ", 100, 0)
	assert.False(ok)

	// Free a slot and reallocate
	assert.True(alloc.FreeChunk("chunkA0", 0))
	ok, offset := alloc.AllocateChunk("chunkY", 100, 0)
	assert.True(ok)
	assert.Equal(uint64(0), offset)
}

func TestMathChunkPoolAllocatorPrintStats(t *testing.T) {
	// Use the math-only version for testability
	slotCounts := []int{3, 2}
	chunkSizes := []uint64{100, 50}
	alloc := NewMathChunkPoolAllocator(slotCounts, chunkSizes)

	// Allocate some chunks
	alloc.AllocateChunk("chunkA", 100, 0)
	alloc.AllocateChunk("chunkB", 100, 0)
	alloc.AllocateChunk("chunkC", 100, 0)
	alloc.AllocateChunk("chunkD", 50, 1)

	// Print stats (simulate what FixedChunkPoolAllocator would do)
	t.Log("--- Detailed Stats ---")
	t.Log(allocStatsString(alloc))
	t.Log("--- Memory Map ---")
	t.Log(allocMemoryMapString(alloc))
}

func TestMathChunkPoolAllocatorThreadSafety(t *testing.T) {
	assert := assert.New(t)

	slotCounts := []int{10}
	chunkSizes := []uint64{100}
	alloc := NewMathChunkPoolAllocator(slotCounts, chunkSizes)
	N := 100
	var wg sync.WaitGroup
	for i := range N {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "chunk" + strconv.Itoa(i%10)
			alloc.AllocateChunk(id, 100, 0)
			alloc.FreeChunk(id, 0)
		}(i)
	}
	wg.Wait()
	// All slots should be free
	for i := range slotCounts[0] {
		id := "chunk" + strconv.Itoa(i)
		assert.False(alloc.FreeChunk(id, 0))
	}
}

// Local math-only deferred free list for test

type mathDeferredFreeList struct {
	chunks []struct {
		chunkID  string
		lodLevel int
	}
}

func (d *mathDeferredFreeList) Add(chunkID string, lodLevel int) {
	d.chunks = append(d.chunks, struct {
		chunkID  string
		lodLevel int
	}{chunkID, lodLevel})
}
func (d *mathDeferredFreeList) ProcessAll(allocator *MathChunkPoolAllocator) int {
	count := 0
	for _, chunk := range d.chunks {
		if allocator.FreeChunk(chunk.chunkID, chunk.lodLevel) {
			count++
		}
	}
	d.chunks = d.chunks[:0]
	return count
}
func (d *mathDeferredFreeList) Count() int { return len(d.chunks) }

func TestMathChunkPoolAllocatorDeferredFreeList(t *testing.T) {
	assert := assert.New(t)

	slotCounts := []int{3}
	chunkSizes := []uint64{100}
	alloc := NewMathChunkPoolAllocator(slotCounts, chunkSizes)
	deferred := &mathDeferredFreeList{}
	alloc.AllocateChunk("chunkA", 100, 0)
	alloc.AllocateChunk("chunkB", 100, 0)
	deferred.Add("chunkA", 0)
	deferred.Add("chunkB", 0)
	count := deferred.ProcessAll(alloc)
	assert.Equal(2, count)
	assert.Equal(0, deferred.Count())
}

func TestMathChunkPoolAllocatorEdgeCases(t *testing.T) {
	assert := assert.New(t)

	slotCounts := []int{2}
	chunkSizes := []uint64{100}
	alloc := NewMathChunkPoolAllocator(slotCounts, chunkSizes)
	assert.False(alloc.FreeChunk("foo", 1))
	ok, _ := alloc.AllocateChunk("foo", 100, 1)
	assert.False(ok)
	ok, _ = alloc.AllocateChunk("foo", 200, 0)
	assert.False(ok)
}

func TestMathChunkPoolAllocatorStatsAndVisualization(t *testing.T) {
	assert := assert.New(t)

	slotCounts := []int{2}
	chunkSizes := []uint64{100}
	alloc := NewMathChunkPoolAllocator(slotCounts, chunkSizes)
	alloc.AllocateChunk("chunkA", 100, 0)
	stats := allocStatsString(alloc)
	assert.Contains(stats, "Used: 1")
	memMap := allocMemoryMapString(alloc)
	assert.Contains(memMap, "A")
}

// Helper functions to simulate the output of GetDetailedStats and VisualizeMemoryMap for MathChunkPoolAllocator
func allocStatsString(a *MathChunkPoolAllocator) string {
	var sb strings.Builder
	sb.WriteString("MathChunkPoolAllocator Statistics:\n")
	for i, bucket := range a.buckets {
		sb.WriteString(fmt.Sprintf("Bucket %d: %d slots, chunk size %d\n", i, bucket.slotCount, bucket.chunkSize))
		sb.WriteString(fmt.Sprintf("  Used: %d, Free: %d\n", len(bucket.allocations), bucket.slotCount-len(bucket.allocations)))
	}
	return sb.String()
}

func allocMemoryMapString(a *MathChunkPoolAllocator) string {
	var sb strings.Builder
	for i, bucket := range a.buckets {
		sb.WriteString(fmt.Sprintf("Bucket %d: [", i))
		for j := range bucket.slotCount {
			if bucket.bitset.IsSet(j) {
				sb.WriteString("A")
			} else {
				sb.WriteString("F")
			}
		}
		sb.WriteString("]\n")
	}
	sb.WriteString("Legend: [A]=Allocated, [F]=Free\n")
	return sb.String()
}
