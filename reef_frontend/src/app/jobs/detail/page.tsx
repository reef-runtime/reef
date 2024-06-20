'use client';

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { useNodes } from '@/stores/nodes.store';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useJobs } from '@/stores/job.store';
import classNames from 'classnames';
import { IJob, IJobResultContentType, IJobStatus } from '@/types/job';
import { BanIcon, CogIcon } from 'lucide-react';
import JobStatusIcon from '@/components/job-status';
import JobListItem from '@/components/job-list-item';
import { useEffect, useState } from 'react';
import { useLogs } from '@/stores/log.store';
import { ILogEntry, ILogKind } from '@/types/log';

export default function Page() {
  const { nodes } = useNodes();
  const { jobs } = useJobs();
  const { logs } = useLogs();

  const [job, setJob] = useState<IJob | null>(null);
  const [jobLogs, setJobLogs] = useState<ILogEntry[] | null>(null);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    document.title = `${job?.name} - Reef`;
  }, [job?.name]);

  useEffect(() => {
    const queryParams = new URLSearchParams(window.location.search);
    const jobId = queryParams.get('id');

    setJob(jobs.find((job) => job.id === jobId) ?? null);
    setJobLogs(logs.filter((log) => log.jobId === jobId));
    setInitialized(true);
  }, []);

  if (!initialized) {
    return null;
  }

  if (!job) {
    window.location.href = '/jobs';
    return null;
  }

  return (
    <main className="flex flex-col p-4 md:space-x-4">
      <div className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle className="flex space-x-2 items-center">
              <span>Job Detail Overview</span>
              <JobStatusIcon job={job} />
            </CardTitle>
          </CardHeader>
          <CardContent className="grow">
            <div className="space-y-4">
              <div>
                <h4 className="font-bold">Job ID</h4>
                <p>{job.id}</p>
              </div>
              <div>
                <h4 className="font-bold">Job Name</h4>
                <p>{job.name}</p>
              </div>
              <div>
                <h4 className="font-bold">Submitted</h4>
                <p>{job.submitted}</p>
              </div>
              {job.result && (
                <>
                  <div>
                    <h4 className="font-bold">Result Success</h4>
                    <p>{job.result.success ? 'Yes' : 'No'}</p>
                  </div>
                  <div>
                    <h4 className="font-bold">Result Content Type</h4>
                    <p>{IJobResultContentType[job.result.contentType]}</p>
                  </div>
                  <div>
                    <h4 className="font-bold">Result Created</h4>
                    <p>{new Date(job.result.created).toLocaleString()}</p>
                  </div>
                </>
              )}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="flex space-x-1 items-center">
              <span>Logs</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="grow p-2">
            <div className="bg-black p-2 rounded-sm h-[40vh]">
              {jobLogs
                ?.sort(
                  (a, b) =>
                    new Date(a.created).getTime() -
                    new Date(b.created).getTime()
                )
                .map((log, index) => (
                  <div key={index} className="font-mono text-xs text-white">
                    <span className="text-green-500">
                      {new Date(log.created).toLocaleString()}
                    </span>
                    <span className="text-blue-500">
                      {' '}
                      [{ILogKind[log.kind]}]
                    </span>
                    <span> {log.content}</span>
                  </div>
                ))}
            </div>
          </CardContent>
        </Card>
      </div>
      <div className="flex flex-col xl:grid gap-4 grow w-full"></div>
    </main>
  );
}
