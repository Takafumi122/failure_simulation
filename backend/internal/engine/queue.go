package engine

import (
	"container/heap"

	"failurelab/backend/internal/model"
)

// SimEvent represents an internal discrete event scheduled on the priority queue.
type SimEvent struct {
	SequenceID  int64
	TimestampMs float64
	Type        model.EventType
	RequestID   int
	SpanID      int64
	ServiceID   string
	Attempt     int
	Reason      model.FailureReason
	Index       int // Maintained by container/heap
}

// EventPriorityQueue implements heap.Interface and orders events by TimestampMs ascending,
// breaking ties deterministically by SequenceID ascending.
type EventPriorityQueue []*SimEvent

func (pq EventPriorityQueue) Len() int { return len(pq) }

func (pq EventPriorityQueue) Less(i, j int) bool {
	if pq[i].TimestampMs == pq[j].TimestampMs {
		return pq[i].SequenceID < pq[j].SequenceID
	}
	return pq[i].TimestampMs < pq[j].TimestampMs
}

func (pq EventPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *EventPriorityQueue) Push(x any) {
	n := len(*pq)
	item := x.(*SimEvent)
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *EventPriorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.Index = -1
	*pq = old[0 : n-1]
	return item
}

// NewEventQueue initializes an empty priority queue.
func NewEventQueue() *EventPriorityQueue {
	pq := make(EventPriorityQueue, 0, 1024)
	heap.Init(&pq)
	return &pq
}
