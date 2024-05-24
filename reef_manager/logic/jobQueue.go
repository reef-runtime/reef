package logic

import (
	"container/heap"
	"sync"

	"github.com/reef-runtime/reef/reef_manager/database"
)

//
// Extermal API for the queue.
//

type JobQueue struct {
	pq   priorityQueue
	len  uint64
	lock sync.RWMutex
}

func NewJobQueue() JobQueue {
	pq := make(priorityQueue, 0)

	heap.Init(&pq)

	return JobQueue{
		pq:   pq,
		len:  0,
		lock: sync.RWMutex{},
	}
}

func (j *JobQueue) Push(job database.Job) {
	j.lock.Lock()
	defer j.lock.Unlock()

	heap.Push(&j.pq, queuedJob{
		Job: job,
	})
	j.len++
}

func (j *JobQueue) Pop() (job database.Job, found bool) {
	j.lock.Lock()
	defer j.lock.Unlock()

	if j.len == 0 {
		return job, false
	}

	item := heap.Pop(&j.pq)
	j.len--

	return item.(*queuedItem[prioritizable]).Inner.(queuedJob).Job, true
}

func (j *JobQueue) Delete(jobID string) (found bool) {
	j.lock.Lock()
	defer j.lock.Unlock()

	const notFound = -1

	// Find index of item to be removed.
	deleteIndex := notFound

	for currIdx, job := range j.pq {
		if job.Inner.(queuedJob).Job.ID == jobID {
			deleteIndex = currIdx
			break
		}
	}

	if deleteIndex == notFound {
		return false
	}

	heap.Remove(&j.pq, deleteIndex)

	return false
}

func (j *JobQueue) IsEmpty() bool {
	j.lock.RLock()
	defer j.lock.RUnlock()

	return j.len == 0
}

func (j *JobQueue) Len() uint64 {
	j.lock.RLock()
	defer j.lock.RUnlock()

	return j.len
}

//
// Internal code of the priority queue.
//

type prioritizable interface {
	IsHigherThan(other prioritizable) bool
}

// An Item is something we manage in a priority queue.
type queuedItem[T any] struct {
	Inner T
	// The index is needed by update and is maintained by the heap.Interface methods.
	index int // The index of the item in the heap.
}

// A priorityQueue implements heap.Interface and holds Items.
type priorityQueue []*queuedItem[prioritizable]

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].Inner.IsHigherThan(pq[j].Inner)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(elementAny any) {
	element := elementAny.(prioritizable)

	n := len(*pq)
	item := queuedItem[prioritizable]{
		Inner: element,
		index: n,
	}
	*pq = append(*pq, &item)
}

func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// update modifies the priority and value of an Item in the queue.
func (pq *priorityQueue) update(item *queuedItem[prioritizable], inner prioritizable) {
	item.Inner = inner
	heap.Fix(pq, item.index)
}
