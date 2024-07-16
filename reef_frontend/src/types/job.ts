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
  // Parse the data as a 32-bit signed little-endian integer.
  // The server guarantees that the length of the `content` array is always 4 elements (= 32 bits).
  ContentTypeI32 = 0,
  // Raw binary data, format as HEX with the option to display it as *lossy* UTF-8.
  ContentTypeRawBytes,
  // Parse the data as an UTF-8 string but don't do anything fancy with it.
  ContentTypeStringPlain,
  // Parse the data as an UTF-8 string and format it as JSON (pretty-print).
  ContentTypeStringJSON,
}

export function displayResultContentType(kind: IJobResultContentType): string {
  switch (kind) {
    case IJobResultContentType.ContentTypeI32:
      return '32-bit signed int';
    case IJobResultContentType.ContentTypeRawBytes:
      return 'Untyped Bytes';
    case IJobResultContentType.ContentTypeStringPlain:
      return 'String';
    case IJobResultContentType.ContentTypeStringJSON:
      return 'JSON';
    default:
      throw 'Another content type was added without updating this code';
  }
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
  // Arbitrary name of the job, should be displayed with an ellipsis
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
  datasetId: string;
  // Session ID / hash of the job's owner.
  owner: string;

  // TODO: comment on these (nuz).
  logs: ILogEntry[];
  progress: number;
  wasmId: string;
}

export function displayJobStatus(job: IJob | null | undefined): string {
  if (!job) {
    return 'NO JOB AVAILABLE';
  }

  switch (job.status) {
    case IJobStatus.StatusQueued:
      return 'QUEUED';
    case IJobStatus.StatusStarting:
      return 'STARTING';
    case IJobStatus.StatusRunning:
      return `RUNNING ${Math.floor(job.progress * 100)}%`;
    case IJobStatus.StatusDone:
      return job.result?.success ? 'SUCCESS' : 'FAILURE';
  }
}
