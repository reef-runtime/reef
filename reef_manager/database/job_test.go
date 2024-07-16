package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var dummyDS = Dataset{
	Id:   "dummy",
	Name: "Dummy Dataset",
	Size: 42,
}

func AddDummyDS(t *testing.T) {
	_, err := AddDataset(dummyDS)
	assert.NoError(t, err)
}

func TestJobResults(t *testing.T) {
	AddDummyDS(t)

	now := time.Now()

	const jobID = "testid"

	datasets, err := ListDatasets()
	assert.NoError(t, err)
	assert.NotEmpty(t, datasets)

	job := JobTableData{
		Id:        jobID,
		Name:      "",
		Submitted: now,
		WasmId:    "",
		DatasetId: datasets[0].Id,
		Owner:     "",
	}

	err = AddJob(job)
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
