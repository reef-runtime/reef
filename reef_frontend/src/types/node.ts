//
// Reef data / schema definition file for job logs.
//

// If this is `null`, no job is being executed on the worker.
export type IJobID = string | null;

export interface INodeInfo {
  // IP address of the node.
  endpointIP: string;
  // Name or hostname of the node.
  name: string;
  // The amount of workers for a node.
  // It is unlikely (HOWEVER NOT GUARANTEED) that this exceeds 64.
  numWorkers: number;
}

// Nodes can not be added manually as the connection is initiated by the node.
export interface INode {
  info: INodeInfo;
  // RFC 3339 time format with sub-second precision.
  // Last time that the node was alive.
  lastPing: string;
  // Unique ID of this node, is guaranteed to be 64 characters long.
  id: string;
  // Length of this array is guaranteed to be `info.numWorkers`.
  // Maps a node's worker to a job that is being executed.
  workerState: IJobID[];
}
