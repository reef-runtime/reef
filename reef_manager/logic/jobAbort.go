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

func MessageToNodeAbortJob(jobID string) ([]byte, error) {
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

	if err := abortMsg.SetJobId(jobID); err != nil {
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

func (m *JobManagerT) AbortJob(jobID string) (found bool, err error) {
	job, found := m.NonFinishedJobs.Get(jobID)
	if !found {
		return false, nil
	}

	switch job.Data.Status {
	case database.StatusQueued:
		return m.abortQueuedJob(jobID)
	case database.StatusStarting, database.StatusRunning:
		// If there is no executing node, something bad happened.
		if job.WorkerNodeID == nil {
			log.Errorf("Possible internal state corruption: non-queued job `%s` has no worker node", jobID)
			return false, nil
		}
		nodeID := *job.WorkerNodeID

		// Inform the node that the job is to be killed.
		// Do not actually delete the job but retain the output.
		node, found := m.Nodes.Get(nodeID)
		if !found {
			return false, fmt.Errorf(
				"job says its running on node `%s`, which does not exist",
				IDToString(nodeID),
			)
		}

		abortMsg, err := MessageToNodeAbortJob(jobID)
		if err != nil {
			return false, err
		}

		// If this fails, the connection to the node dropped during the kill request.
		// In this case, drop the node and execute same behavior as if the job was queued .
		if err := node.Conn.WriteMessage(websocket.BinaryMessage, abortMsg); err != nil {
			errMsg := fmt.Sprintf(
				"node `%s` dropped connection whilst job should be killed and could not be dropped",
				IDToString(nodeID),
			)

			if !m.DropNode(nodeID) {
				return false, errors.New(errMsg)
			}

			if _, err := m.abortQueuedJob(jobID); err != nil {
				return false, fmt.Errorf("%s: %s", errMsg, err.Error())
			}

			// If the node could be dropped successfully, consider this a successful abortion.
			return true, nil
		}
	case database.StatusDone:
		panic("unreachable: a `done` job is never in the list of not-done jobs")
	}

	return true, nil
}

func (m *JobManagerT) abortQueuedJob(jobID string) (found bool, err error) {
	_, found = m.NonFinishedJobs.Delete(jobID)
	if !found {
		return false, nil
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
