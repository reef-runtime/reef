'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Separator } from '@/components/ui/separator';

import JobProgress from '@/components/job-progress';
import JobOutput from '@/components/job-output';
import JobStatusIcon from '@/components/job-status';

import { useJobs } from '@/stores/job.store';
import { IJob, IJobResultContentType, IJobStatus } from '@/types/job';
import { GetSocket, topicSingleJob } from '@/lib/websocket';
import { useReefSession } from '@/stores/session.store';

export default function Page() {
  const { jobs, setJobs } = useJobs();
  const { session, fetchSession } = useReefSession();

  const [job, setJob] = useState<IJob | null>(null);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    document.title = `${job?.name} - Reef`;
  }, [job?.name]);

  /* eslint-disable react-hooks/exhaustive-deps */
  useEffect(() => {
    fetchSession(null);

    const queryParams = new URLSearchParams(window.location.search);
    const jobId = queryParams.get('id');

    if (!jobId) {
      throw 'bug';
      // TODO: redirect to 404 page
      return;
    }

    const sock = GetSocket();
    sock.unsubscribeAll();

    sock.subscribe(topicSingleJob(jobId), (res) => {
      setJobs([res.data]);

      setJob(res.data);
      setInitialized(true);
    });
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  if (!initialized || !job) {
    return null;
  }

  const killJob = async () => {
    let killResponse = await fetch('/api/job/abort', {
      method: 'DELETE',
      body: JSON.stringify({
        id: job.id,
      }),
      credentials: 'include',
    });

    console.log(`Response ok: ${killResponse.ok}`);
  };

  return (
    <main className="flex flex-col-reverse xl:flex-row p-4 gap-4 w-full xl:h-full">
      <Card className="grow flex flex-col">
        <Tabs
          defaultValue="logs"
          className="grow overflow-hidden flex flex-col"
        >
          <CardHeader>
            <TabsList className="p-0 max-w-min">
              <TabsTrigger value="logs" className="rounded py-2">
                <CardTitle>Logs</CardTitle>
              </TabsTrigger>
              <TabsTrigger value="result" className="rounded py-2">
                <CardTitle>Result</CardTitle>
              </TabsTrigger>
            </TabsList>
          </CardHeader>

          <Separator />

          <TabsContent value="logs" className="grow my-6 overflow-hidden">
            <CardContent className="h-full m-6 mt-0 p-2 dark:bg-stone-950 rounded ">
              <JobOutput job={job} compact={false}></JobOutput>
            </CardContent>
          </TabsContent>
          <TabsContent value="result" className="mt-6">
            <CardContent className="grow">
              {job.result ? (
                <div>Job results here</div>
              ) : (
                <div>No Results available yet.</div>
              )}
            </CardContent>
          </TabsContent>
        </Tabs>
      </Card>

      <Card className="w-full xl:w-[400px]">
        <CardHeader>
          <CardTitle className="flex space-x-2 items-center">
            <span>Job Details</span>
            <JobStatusIcon job={job} />
          </CardTitle>
        </CardHeader>
        <CardContent className="grow space-y-4">
          <JobProgress job={job} />
          <div>
            <h4 className="font-bold">Job ID</h4>
            <p className="overflow-hidden text-ellipsis">{job.id}</p>
          </div>
          <div>
            <h4 className="font-bold">Job Name</h4>
            <p className="overflow-hidden text-ellipsis">{job.name}</p>
          </div>
          <div>
            <h4 className="font-bold">Dataset ID</h4>
            <p className="overflow-hidden text-ellipsis">{job.datasetId}</p>
          </div>
          <div>
            <h4 className="font-bold">Submitted</h4>
            <p className="overflow-hidden text-ellipsis">
              {new Date(job.submitted).toLocaleString()}
            </p>
          </div>
          <div>
            <h4 className="font-bold">Progress</h4>
            <p className="overflow-hidden text-ellipsis">
              {Math.floor(job.progress * 10000) / 100}%
            </p>
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

          {(job.owner === session.id || session.isAdmin) &&
          job.status !== IJobStatus.StatusDone ? (
            <Button variant={'destructive'} onClick={killJob}>
              Kill Job
            </Button>
          ) : null}
        </CardContent>
      </Card>
    </main>
  );
}
