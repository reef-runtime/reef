package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobResults(t *testing.T) {
	now := time.Now()

	const jobID = "testid"

	job := JobTableData{
		ID:        jobID,
		Name:      "",
		Submitted: now,
		WasmID:    "",
		DatasetID: "",
	}

	err := AddJob(job)
	assert.NoError(t, err)

	res := Result{
		Success:     false,
		JobID:       jobID,
		Content:     []byte{1, 2, 3},
		ContentType: Bytes,
		Created:     now,
	}

	err = SaveResult(res)
	assert.NoError(t, err)

	result, found, err := GetResult(jobID)
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, result.ContentType, Bytes)
	assert.Equal(t, result.Content, []byte{1, 2, 3})
	assert.Equal(t, result.JobID, jobID)
}
