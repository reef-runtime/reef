package logic

import (
	"errors"
	"fmt"
	"time"

	"capnproto.org/go/capnp/v3"
	"github.com/gorilla/websocket"
	"github.com/reef-runtime/reef/reef_manager/database"
	node "github.com/reef-runtime/reef/reef_protocol_node"
)

const jobAbortMessage = "Job was aborted."

//
// Job Abortion.
//

func MessageToNodeAbortJob(jobId string) ([]byte, error) {
	msg, seg, err := capnp.NewMessage(capnp.SingleSegment(nil))
	if err != nil {
		return nil, err
	}

	toNodeMsg, err := node.NewRootMessageToNode(seg)
	if err != nil {
		return nil, err
	}

	toNodeMsg.SetKind(node.MessageToNodeKind_abortJob)

	//
	// Nested.
	//

	abortMsg, err := node.NewJobAbortMessage(seg)
	if err != nil {
		return nil, err
	}

	if err := abortMsg.SetJobId(jobId); err != nil {
		return nil, err
	}

	if err := toNodeMsg.Body().SetAbortJob(abortMsg); err != nil {
		return nil, err
	}

	return msg.Marshal()
}

//
// Kills / Aborts the job.
// NOTE: this does not delete the job but puts it into an error-state.
// For queued jobs, no communication with a node is required.
//

func (m *JobManagerT) AbortJob(jobId string) (found bool, err error) {
	const abortLogMsg = "This job will be aborted soon."

	job, found := m.NonFinishedJobs.Get(jobId)
	if !found {
		return false, nil
	}

	job.Lock.RLock()
	status := job.Data.Status
	job.Lock.RUnlock()

	switch status {
	case StatusQueued:
		if found, err := m.abortQueuedJob(jobId, abortLogMsg); !found || err != nil {
			return found, err
		}
	case StatusStarting, StatusRunning:
		// If there is no executing node, something bad happened.
		job.Lock.RLock()
		workerNodeID := job.Data.WorkerNodeID
		job.Lock.RUnlock()

		if workerNodeID == nil {
			log.Errorf("Possible internal state corruption: non-queued job `%s` has no worker node", jobId)
			return false, nil
		}
		nodeID := *workerNodeID

		// Inform the node that the job is to be killed.
		// Do not actually delete the job but retain the output.
		node, found := m.Nodes.Get(nodeID)
		if !found {
			return false, fmt.Errorf(
				"job says its running on node `%s`, which does not exist",
				IdToString(nodeID),
			)
		}

		abortMsg, err := MessageToNodeAbortJob(jobId)
		if err != nil {
			return false, err
		}

		// If this fails, the connection to the node dropped during the kill request.
		// In this case, drop the node and execute same behavior as if the job was queued .
		node.Lock.Lock()
		err = node.Data.Conn.WriteMessage(websocket.BinaryMessage, abortMsg)
		node.Lock.Unlock()

		if err != nil {
			errMsg := fmt.Sprintf(
				"node `%s` dropped connection whilst job should be killed and could not be dropped",
				IdToString(nodeID),
			)

			if !m.DropNode(nodeID) {
				return false, errors.New(errMsg)
			}

			if _, err := m.abortQueuedJob(jobId, abortLogMsg); err != nil {
				return false, fmt.Errorf("%s: %s", errMsg, err.Error())
			}

			// If the node could be dropped successfully, consider this a successful abortion.
			return true, nil
		}

		// Add log message.
		if err := database.AddLog(database.JobLog{
			Kind:    database.LogKindSystem,
			Created: time.Now(),
			Content: abortLogMsg,
			JobId:   jobId,
		}); err != nil {
			return false, err
		}

	case StatusDone:
		panic("unreachable: a `done` job is never in the list of not-done jobs")
	}

	// Notify UI about change.
	m.updateSingleJobState(jobId)

	return true, nil
}

func (m *JobManagerT) abortQueuedJob(jobID string, logMessage string) (found bool, err error) {
	_, found = m.NonFinishedJobs.Delete(jobID)
	if !found {
		return false, nil
	}

	if err := database.AddLog(database.JobLog{
		Kind:    database.LogKindSystem,
		Created: time.Now(),
		Content: logMessage,
		JobId:   jobID,
	}); err != nil {
		return false, err
	}

	if err := database.SaveResult(database.Result{
		Success:     false,
		JobID:       jobID,
		Content:     []byte(jobAbortMessage),
		ContentType: database.StringPlain,
		Created:     time.Now(),
	}); err != nil {
		return false, err
	}

	return true, nil
}
