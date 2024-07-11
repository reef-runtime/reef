'use client';

import { useEffect, useState } from 'react';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

import JobProgress from '@/components/job-progress';
import JobOutput from '@/components/job-output';
import JobStatusIcon from '@/components/job-status';

import { useJobs } from '@/stores/job.store';
import { IJob, IJobResultContentType } from '@/types/job';
import { GetSocket, topicSingleJob } from '@/lib/websocket';

export default function Page() {
  const { jobs, setJobs } = useJobs();

  const [job, setJob] = useState<IJob | null>(null);
  const [initialized, setInitialized] = useState(false);

  useEffect(() => {
    document.title = `${job?.name} - Reef`;
  }, [job?.name]);

  /* eslint-disable react-hooks/exhaustive-deps */
  useEffect(() => {
    const queryParams = new URLSearchParams(window.location.search);
    const jobId = queryParams.get('id');

    console.log(jobId);

    if (!jobId) {
      throw 'bug';
      // TODO: redirect to 404 page
      return;
    }

    const sock = GetSocket();
    sock.unsubscribeAll();

    sock.subscribe(topicSingleJob(jobId), (res) => {
      console.dir(res.data);
      setJobs([res.data]);

      setJob(res.data);
      setInitialized(true);
    });
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  if (!initialized || !job) {
    return null;
  }

  return (
    <main className="flex flex-col-reverse xl:flex-row p-4 gap-4 w-full">
      <Card className="grow">
        <CardHeader>
          <CardTitle>Logs</CardTitle>
        </CardHeader>
        <CardContent className="grow m-6 mt-0 p-2 bg-stone-950 rounded">
          <JobOutput job={job} compact={false}></JobOutput>
        </CardContent>
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
        </CardContent>
      </Card>
    </main>
  );
}
