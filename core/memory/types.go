package memory

import (
	"sync"

	vk "github.com/vulkan-go/vulkan"
)

// AllocatorStats holds memory usage statistics for an allocator.
type AllocatorStats struct {
	TotalSize   uint32
	UsedSize    uint32
	FreeSize    uint32
	Allocations int
}

// AllocationInfo holds buffer, offset, and size for an allocated chunk.
type AllocationInfo struct {
	Buffer vk.Buffer
	Offset vk.DeviceSize
	Size   vk.DeviceSize
}

// SlotStack is a simple stack-based free list for slot indices.
type SlotStack struct {
	stack []int
	mu    sync.Mutex
}

// NewFreeList creates a new FreeList with the given capacity.
func NewFreeList(capacity int) *SlotStack {
	stack := make([]int, capacity)
	for i := range capacity {
		stack[i] = capacity - 1 - i
	}
	return &SlotStack{stack: stack}
}

// Pop removes and returns the top element from the free list.
func (s *SlotStack) Pop() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.stack) == 0 {
		return 0, false
	}
	idx := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return idx, true
}

// Push adds an element back to the free list.
func (s *SlotStack) Push(idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stack = append(s.stack, idx)
}
