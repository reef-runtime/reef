import { IJob, IJobResultContentType, IJobStatus } from '../types/job';


export const mockJobs: IJob[] = [
    {
        id: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
        name: 'Data Processing Job',
        submitted: '2023-10-01T10:00:00Z',
        status: IJobStatus.StatusQueued,
        datasetId: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
    },
    {
        id: 'abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890',
        name: 'Genome Processing',
        submitted: '2023-10-01T10:05:00Z',
        status: IJobStatus.StatusStarting,
        datasetId: '1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef',
    },
    {
        id: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
        name: 'Weather Data Analysis',
        submitted: '2023-10-01T10:10:00Z',
        status: IJobStatus.StatusRunning,
        datasetId: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
    },
    {
        id: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
        name: 'Text Analysis Job',
        submitted: '2023-10-01T10:15:00Z',
        status: IJobStatus.StatusDone,
        result: {
            success: true,
            jobID: '4567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234',
            content: new Uint8Array([
                116, 101, 120, 116, 32, 97, 110, 97, 108, 121, 115, 105, 115,
            ]),
            contentType: IJobResultContentType.ContentTypeStringPlain,
            created: '2023-10-01T10:20:00Z',
        },
        datasetId: '7890abcdef1234567890abcdef1234567890abcdef1234567890abcdef123456',
    },
    {
        id: 'todo-efabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef',
        name: 'TODO APP Create Task',
        submitted: '2023-10-01T10:25:00Z',
        status: IJobStatus.StatusDone,
        result: {
            jobID: "todo-efabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef",
            success: false,
            content: new TextEncoder().encode("foo.hms: 12:42: something bad happened"),
            contentType: IJobResultContentType.ContentTypeStringPlain,
            created: '2023-10-01T10:20:00Z',
        },
        datasetId: 'abcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdef',
    },
];
