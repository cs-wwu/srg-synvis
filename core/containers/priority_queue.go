package containers

import (
	"container/heap"
	"sync"
)

// PriorityQueue implements a generic priority queue for jobs
type PriorityQueue[T any] struct {
	items       []PriorityItem[T]
	getPriority func(T) float32
	mutex       sync.Mutex
}

type PriorityItem[T any] struct {
	Value    T
	Priority float32
	Index    int
}

// NewPriorityQueue creates a new priority queue with higher priority items first
func NewPriorityQueue[T any](getPriority func(T) float32) *PriorityQueue[T] {
	pq := &PriorityQueue[T]{
		items:       make([]PriorityItem[T], 0),
		getPriority: getPriority,
	}
	heap.Init(pq)
	return pq
}

func (pq *PriorityQueue[T]) Len() int { return len(pq.items) }

func (pq *PriorityQueue[T]) Less(i, j int) bool {
	// Higher priority first
	return pq.items[i].Priority > pq.items[j].Priority
}

func (pq *PriorityQueue[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].Index = i
	pq.items[j].Index = j
}

func (pq *PriorityQueue[T]) Push(x any) {
	item := x.(PriorityItem[T])
	item.Index = len(pq.items)
	pq.items = append(pq.items, item)
}

func (pq *PriorityQueue[T]) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	item.Index = -1
	pq.items = old[0 : n-1]
	return item
}

// Enqueue adds an item to the priority queue
func (pq *PriorityQueue[T]) Enqueue(value T) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	priority := pq.getPriority(value)
	item := PriorityItem[T]{
		Value:    value,
		Priority: priority,
	}
	heap.Push(pq, item)
}

// Dequeue removes and returns the highest priority item
func (pq *PriorityQueue[T]) Dequeue() (T, bool) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	var zero T
	if pq.Len() == 0 {
		return zero, false
	}

	item := heap.Pop(pq).(PriorityItem[T])
	return item.Value, true
}

// Size returns the number of items in the queue
func (pq *PriorityQueue[T]) Size() int {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	return pq.Len()
}

// Clear removes all items from the queue
func (pq *PriorityQueue[T]) Clear() {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()
	pq.items = pq.items[:0]
	heap.Init(pq)
}
