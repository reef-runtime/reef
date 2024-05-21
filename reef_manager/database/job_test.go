package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobResults(t *testing.T) {
	job := Job{
		ID:        "testid",
		Name:      "",
		Submitted: time.Now(),
		Status:    StatusFinished,
	}

	assert.NoError(t, AddJob(job))

	now := time.Now()
	res := Result{
		ID:          "testid",
		Content:     []byte{1, 2, 3},
		ContentType: Bytes,
		Created:     now,
	}
	assert.NoError(t, SaveResult(res))

	result, _, err := GetResult("testid")
	
	assert.NoError(t, err)

	assert.Equal(t, result.ContentType, Bytes)
	assert.Equal(t, result.Content, []byte{1, 2, 3})
	assert.Equal(t, result.ID, "testid")
}
