package containers

import (
	"container/heap"
	"sync"
)

// PriorityQueue implements a generic priority queue for jobs
type PriorityQueue[T any] struct {
	storage     queueStorage[T]
	getPriority func(T) float32
	mutex       sync.Mutex
}

// Implements `heap.Interface`, ie, `Len`, `Less`, `Swap`, `Push`, and `Pop`
type queueStorage[T any] struct {
	items []PriorityItem[T]
}

type PriorityItem[T any] struct {
	Value    T
	Priority float32
	Index    int
}

// NewPriorityQueue creates a new priority queue with higher priority items first
func NewPriorityQueue[T any](getPriority func(T) float32) *PriorityQueue[T] {
	pq := &PriorityQueue[T]{
		storage: queueStorage[T]{
			items: make([]PriorityItem[T], 0),
		},
		getPriority: getPriority,
	}
	heap.Init(&pq.storage)
	return pq
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
	heap.Push(&pq.storage, item)
}

// Dequeue removes and returns the highest priority item
func (pq *PriorityQueue[T]) Dequeue() (item T, exists bool) {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	exists = pq.storage.Len() > 0
	if exists {
		item = heap.Pop(&pq.storage).(PriorityItem[T]).Value
	}
	return
}

// Size returns the number of items in the queue
func (pq *PriorityQueue[T]) Size() int {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	return pq.storage.Len()
}

// Clear removes all items from the queue
func (pq *PriorityQueue[T]) Clear() {
	pq.mutex.Lock()
	defer pq.mutex.Unlock()

	pq.storage.items = pq.storage.items[:0]
}

func (pq *queueStorage[T]) Len() int { return len(pq.items) }

func (pq *queueStorage[T]) Less(i, j int) bool {
	// Higher priority first
	return pq.items[i].Priority > pq.items[j].Priority
}

func (pq *queueStorage[T]) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].Index = i
	pq.items[j].Index = j
}

func (pq *queueStorage[T]) Push(x any) {
	item := x.(PriorityItem[T])
	item.Index = len(pq.items)
	pq.items = append(pq.items, item)
}

func (pq *queueStorage[T]) Pop() any {
	old := pq.items
	n := len(old)
	item := old[n-1]
	item.Index = -1
	pq.items = old[0 : n-1]
	return item
}
