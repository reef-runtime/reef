package logic

import (
	"fmt"
	"testing"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
)

func TestJobQueue(t *testing.T) {
	baseSubmit := time.Now()

	items := []database.Job{
		{
			ID:        [32]byte{123, 121, 0101},
			Name:      "Analyze weather data",
			Submitted: baseSubmit.Add(time.Hour * 3),
			Status:    database.StatusQueued,
		},
		{
			ID:        [32]byte{123, 45, 67, 13},
			Name:      "Mine some Coin",
			Submitted: baseSubmit.Add(time.Hour * 3),
			Status:    database.StatusQueued,
		},
		{
			ID:        [32]byte{47, 11, 42, 69},
			Name:      "Calculate Fibonacci",
			Submitted: baseSubmit.Add(time.Hour * 2),
			Status:    database.StatusQueued,
		},
	}

	jobs := NewJobQueue()

	// i := 0
	// for idx, elem := range items {
	// 	pq[i] = &Item{
	// 		value:    value,
	// 		priority: priority,
	// 		index:    i,
	// 	}
	// 	i++
	// }

	// Insert a new item and then modify its priority.
	// item := &Item{
	// 	value:    "orange",
	// 	priority: 1,
	// }

	for _, elem := range items {
		jobs.Push(elem)
	}

	// pq.update(item, item.value, 5)

	// Take the items out; they arrive in decreasing priority order.
	// for pq.Len() > 0 {
	// 	item := heap.Pop(&pq).(*queuedItem[prioritizable]).Inner.(queuedJob)
	// 	fmt.Printf("name=%s | %v | %v", item.Job.Name, item.Job.ID, item.Job.Submitted)
	// }

	for {
		job, exists := jobs.Pop()
		if !exists {
			break
		}

		fmt.Printf("JOB: %b\n", job.ID)
	}
}
