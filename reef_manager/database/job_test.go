package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobResults(t *testing.T) {
	now := time.Now()

	const jobId = "testid"

	job := JobTableData{
		Id:        jobId,
		Name:      "",
		Submitted: now,
		WasmId:    "",
		DatasetId: "",
	}

	err := AddJob(job)
	assert.NoError(t, err)

	res := Result{
		Success:     false,
		JobId:       jobId,
		Content:     []byte{1, 2, 3},
		ContentType: Bytes,
		Created:     now,
	}

	err = SaveResult(res)
	assert.NoError(t, err)

	result, found, err := GetResult(jobId)
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, result.ContentType, Bytes)
	assert.Equal(t, result.Content, []byte{1, 2, 3})
	assert.Equal(t, result.JobId, jobId)
}
