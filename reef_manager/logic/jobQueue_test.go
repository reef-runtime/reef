package logic

import (
	"fmt"
	"testing"
	"time"

	"github.com/reef-runtime/reef/reef_manager/database"
	"github.com/stretchr/testify/assert"
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
			Submitted: baseSubmit.Add(time.Hour * 2),
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

	for _, elem := range items {
		jobs.Push(elem)
	}

	assert.False(t, jobs.IsEmpty())
	assert.Equal(t, jobs.Len(), uint64(len(items)))
	results := make([]database.Job, 0)
	count := jobs.Len()

	for {
		job, exists := jobs.Pop()
		if !exists {
			break
		}

		fmt.Printf("[test] Job ID: %b\n", job.ID)

		results = append(results, job)
		count--

		assert.Equal(t, jobs.Len(), count)
	}

	assert.Len(t, results, len(items))
	assert.Equal(t, results[0].Name, "Mine some Coin")
	assert.Equal(t, results[1].Name, "Calculate Fibonacci")
	assert.Equal(t, results[2].Name, "Analyze weather data")
	assert.True(t, jobs.IsEmpty())
	assert.Equal(t, jobs.Len(), uint64(0))
}
