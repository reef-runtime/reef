'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
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
// import { useLogs } from '@/stores/log.store';
import { ILogEntry, ILogKind } from '@/types/log';
import { GetSocket, topicSingleJob } from '@/lib/websocket';

export default function Page() {
  const { nodes, setNodes } = useNodes();
  const { jobs, setJobs } = useJobs();
  // const { logs } = useLogs();

  const [job, setJob] = useState<IJob | null>(null);
  // const [jobLogs, setJobLogs] = useState<ILogEntry[] | null>(null);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    document.title = `${job?.name} - Reef`;
  }, [job?.name]);

  useEffect(() => {
    const queryParams = new URLSearchParams(window.location.search);
    const jobId = queryParams.get('id');

    if (!jobId) {
      // TODO: redirect to 404 page
      return;
    }

    setJob(jobs.find((job) => job.id === jobId) ?? null);
    setInitialized(true);

    const sock = GetSocket();
    sock.unsubscribeAll();

    sock.subscribe(topicSingleJob(jobId), (res) => {
      setJobs([res.data]);
    });
  }, []);

  if (!initialized) {
    return null;
  }

  if (!job) {
    window.location.href = '/jobs';
    return null;
  }

  return (
    <main className="flex xl:flex-row p-4 xl:space-x-4 grow">
      <div className="space-y-4 grow flex flex-col">
        <Card className="grow flex flex-col">
          <CardHeader>
            <CardTitle className="flex space-x-1 items-center">
              <span>Logs</span>
            </CardTitle>
          </CardHeader>
          <CardContent className="grow p-2 flex flex-col">
            <div className="bg-black p-2 rounded-sm grow">
              {job.logs
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
      <div className="flex flex-col xl:grid gap-4 w-[400px]">
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
      </div>
    </main>
  );
}
