import { IJob, IJobResultContentType, IJobStatus } from '../types/job';
import { ILogEntry, ILogKind } from '../types/log';
import { INode } from '../types/node';
import { IDataset } from '../types/dataset';

export const mockNodes: INode[] = [
  {
    info: {
      endPointIP: '192.168.1.1',
      name: 'Node-1',
      numWorkers: 4,
    },
    lastPing: '2023-10-01T12:00:00Z',
    id: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    workerState: [
      null,
      '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
      'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
      null,
    ],
  },
  {
    info: {
      endPointIP: '192.168.1.2',
      name: 'Node-2',
      numWorkers: 2,
    },
    lastPing: '2023-10-01T12:05:00Z',
    id: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
    workerState: [
      '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
      null,
    ],
  },
  {
    info: {
      endPointIP: '192.168.1.3',
      name: 'Node-3',
      numWorkers: 3,
    },
    lastPing: '2023-10-01T12:10:00Z',
    id: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
    workerState: [
      null,
      '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
      '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    ],
  },
  {
    info: {
      endPointIP: '192.168.1.4',
      name: 'Node-4',
      numWorkers: 1,
    },
    lastPing: '2023-10-01T12:15:00Z',
    id: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
    workerState: [null],
  },
];

export const mockLogs: ILogEntry[] = [
  {
    kind: ILogKind.LogKindProgram,
    created: '2023-10-01T12:00:00Z',
    content: 'Program started successfully.',
    jobId: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
  },
  {
    kind: ILogKind.LogKindNode,
    created: '2023-10-01T12:05:00Z',
    content: 'Node-1 connected to the network.',
    jobId: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
  },
  {
    kind: ILogKind.LogKindSystem,
    created: '2023-10-01T12:10:00Z',
    content: 'System check completed.',
    jobId: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
  },
  {
    kind: ILogKind.LogKindProgram,
    created: '2023-10-01T12:15:00Z',
    content: 'Job-1 started execution.',
    jobId: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
  },
  {
    kind: ILogKind.LogKindNode,
    created: '2023-10-01T12:20:00Z',
    content: 'Node-2 reported an error.',
    jobId: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
  },
  {
    kind: ILogKind.LogKindSystem,
    created: '2023-10-01T12:25:00Z',
    content: 'System maintenance started.',
    jobId: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
  },
  {
    kind: ILogKind.LogKindProgram,
    created: '2023-10-01T12:30:00Z',
    content: 'Job-2 completed successfully.',
    jobId: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
  },
  {
    kind: ILogKind.LogKindNode,
    created: '2023-10-01T12:35:00Z',
    content: 'Node-3 disconnected from the network.',
    jobId: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
  },
  {
    kind: ILogKind.LogKindSystem,
    created: '2023-10-01T12:40:00Z',
    content: 'System maintenance completed.',
    jobId: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
  },
  {
    kind: ILogKind.LogKindProgram,
    created: '2023-10-01T12:45:00Z',
    content: 'Program terminated.',
    jobId: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
  },
];

export const mockJobs: IJob[] = [
  {
    id: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    name: 'Data Processing Job',
    submitted: '2023-10-01T10:00:00Z',
    status: IJobStatus.StatusQueued,
    datasetId:
      'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
  },
  {
    id: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
    name: 'Image Recognition Job',
    submitted: '2023-10-01T10:05:00Z',
    status: IJobStatus.StatusStarting,
    datasetId:
      '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
  },
  {
    id: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
    name: 'Video Encoding Job',
    submitted: '2023-10-01T10:10:00Z',
    status: IJobStatus.StatusRunning,
    datasetId:
      '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
  },
  {
    id: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
    name: 'Text Analysis Job',
    submitted: '2023-10-01T10:15:00Z',
    status: IJobStatus.StatusDone,
    result: {
      success: true,
      jobID:
        '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
      content: new Uint8Array([
        116, 101, 120, 116, 32, 97, 110, 97, 108, 121, 115, 105, 115,
      ]),
      contentType: IJobResultContentType.ContentTypeStringPlain,
      created: '2023-10-01T10:20:00Z',
    },
    datasetId:
      '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
  },
  {
    id: 'abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef',
    name: 'Machine Learning Model Training',
    submitted: '2023-10-01T10:25:00Z',
    status: IJobStatus.StatusQueued,
    datasetId:
      'abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef',
  },
];

export const mockDatasets: IDataset[] = [
  {
    id: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
    name: 'Dataset for Data Processing Job',
    size: 1048576, // 1 MB
  },
  {
    id: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    name: 'Dataset for Image Recognition Job',
    size: 2097152, // 2 MB
  },
  {
    id: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
    name: 'Dataset for Video Encoding Job',
    size: 3145728, // 3 MB
  },
  {
    id: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
    name: 'Dataset for Text Analysis Job',
    size: 4194304, // 4 MB
  },
  {
    id: 'abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef',
    name: 'Dataset for Machine Learning Model Training',
    size: 5242880, // 5 MB
  },
];
