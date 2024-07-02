//
// Reef data / schema definition file for jobs.
//

import { ILogEntry } from './log';

export enum IJobStatus {
  // Job has not yet started, waiting for free worker.
  // A job will be set back into this state if the executing worker
  // dies and the manager searches for a failover.
  StatusQueued = 0,
  // Job data is being loaded by the assigned worker, no execution yet.
  StatusStarting = 1,
  // Job is executing on its assigned worker node.
  StatusRunning = 2,
  // Actual job status can be queried through the result.
  StatusDone = 3,
}

export enum IJobResultContentType {
  // Parse the data as an UTF-8 string and format it as JSON (pretty-print).
  ContentTypeStringJSON = 0,
  // Parse the data as an UTF-8 string but don't do anything fancy with it.
  ContentTypeStringPlain,
  // Parse the data as a 32-bit signed little-endian integer.
  // The server guarantees that the length of the `content` array is always 4 elements (= 32 bits).
  ContentTypeI32,
  // Raw binary data, format as HEX with the option to display it as *lossy* UTF-8.
  ContentTypeRawBytes,
}

export interface IJobResult {
  // If this is false, the job result's content-type
  // is guaranteed to be `string-plain`.
  success: boolean;
  // ID of the corresponding job.
  jobID: string;
  // Untyped, binary data: the `contentType` field describes how the UI should display this data.
  content: Uint8Array;
  // Describes how the `content` field is to be displayed.
  contentType: IJobResultContentType;
  // When the result was created. Can be used to calculate the runtime of a job.
  // RFC 3339 time format with sub-second precision.
  created: string;
}

export interface IJob {
  // Primary key of each job. Server guarantees that this is unique.
  // SHA256 hash, always 64 characters long.
  id: string;
  // Arbitrary name of the job, should be displayed with an elipsis
  // effect so that long names dont destroy the UI.
  name: string;
  // RFC 3339 time format with sub-second precision.
  // When the job was initially submitted
  submitted: string;
  // Always a valid variant of the enum, display with a name for the status and a matching icon.
  status: IJobStatus;
  // Is ``!= null` as soon as the job has completed execution.
  // The result itself determines whether the job executed successfully or with errors.
  result?: IJobResult;
  // The ID of the attached dataset, always 64 characters long.
  // The backend guarantees that this is always valid.
  datasetId?: string;

  // TODO: comment on these (nuz).
  logs: ILogEntry[];
  progress: number;
  wasmId: string;
}
