package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJobResults(t *testing.T) {
	const testID = "testID"

	now := time.Now()

	job := Job{
		ID:        testID,
		Name:      "",
		Submitted: now,
		Status:    StatusFinished,
	}

	err := AddJob(job)
	assert.NoError(t, err)

	res := Result{
		ID:          testID,
		Content:     []byte{1, 2, 3},
		ContentType: Bytes,
		Created:     now,
	}

	err = SaveResult(res)
	assert.NoError(t, err)

	result, found, err := GetResult(testID)
	assert.NoError(t, err)
	assert.True(t, found)

	assert.Equal(t, result.ContentType, Bytes)
	assert.Equal(t, result.Content, []byte{1, 2, 3})
	assert.Equal(t, result.ID, testID)
}
