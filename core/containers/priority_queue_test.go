package containers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testJob struct {
	id       int
	priority float32
}

func newTestQueue() *PriorityQueue[testJob] {
	return NewPriorityQueue(func(job testJob) float32 { return job.priority })
}

func TestPriorityQueueAddRemoveSingle(t *testing.T) {
	assert := assert.New(t)
	pq := newTestQueue()

	pq.Enqueue(testJob{id: 10, priority: 1})

	item, ok := pq.Dequeue()
	assert.True(ok)
	assert.Equal(item.id, 10)
}

func TestPriorityQueueAddRemoveMultiple(t *testing.T) {
	assert := assert.New(t)
	pq := newTestQueue()

	pq.Enqueue(testJob{id: 1, priority: 1})
	pq.Enqueue(testJob{id: 2, priority: 3})
	pq.Enqueue(testJob{id: 3, priority: 2})

	item, ok := pq.Dequeue()
	assert.True(ok)
	assert.Equal(item.id, 2)

	item, ok = pq.Dequeue()
	assert.True(ok)
	assert.Equal(item.id, 3)

	item, ok = pq.Dequeue()
	assert.True(ok)
	assert.Equal(item.id, 1)
}

func TestPriorityQueueRemoveEmpty(t *testing.T) {
	assert := assert.New(t)
	pq := newTestQueue()

	_, ok := pq.Dequeue()
	assert.False(ok)
}
