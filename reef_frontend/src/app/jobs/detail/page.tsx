'use client';

import { useEffect, useState } from 'react';

import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Separator } from '@/components/ui/separator';

import JobProgress from '@/components/job-progress';
import JobOutput from '@/components/job-output';
import JobStatusIcon, { colorClassForJob } from '@/components/job-status';

import { useJobs } from '@/stores/job.store';
import {
  displayJobStatus,
  displayResultContentType,
  IJob,
  IJobResultContentType,
  IJobStatus,
} from '@/types/job';
import { GetSocket, topicSingleJob } from '@/lib/websocket';
import { useReefSession } from '@/stores/session.store';
import { humanFileSize } from '@/lib/utils';
import classNames from 'classnames';

export default function Page() {
  const { jobs, setJobs } = useJobs();
  const { session, fetchSession } = useReefSession();

  const [job, setJob] = useState<IJob | null>(null);
  const [initialized, setInitialized] = useState(false);
  const [resultContent, setResultContent] = useState<ArrayBuffer | null>(null);

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

  // fetch result if available
  useEffect(() => {
    if (!job?.result) {
      setResultContent(null);
      return;
    }

    fetchResultData();
  }, [job?.result]);

  const fetchResultData = async () => {
    if (!job?.result) throw 'bug';

    let resultResponse = await fetch(`/api/job/result/${job.id}`);
    let resultData = await resultResponse.json();

    // convert base64 to arraybuffer
    let buffer = await (
      await fetch('data:application/binary;base64,' + resultData.content)
    ).arrayBuffer();

    setResultContent(buffer);
  };

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

              {job.result ? (
                <TabsTrigger value="result" className="rounded py-2">
                  <CardTitle>Result</CardTitle>
                </TabsTrigger>
              ) : null}
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
                <div className="space-y-2">
                  <div>
                    <h4 className="font-bold text-lg mt-1">Finished At</h4>
                    <p>{new Date(job.result.created).toLocaleString()}</p>
                  </div>
                  <div>
                    <h4 className="font-bold text-lg mt-1">Content Type</h4>
                    <p>{displayResultContentType(job.result.contentType)}</p>
                  </div>
                  <div>
                    <h4 className="font-bold text-lg mt-1">Result Size</h4>
                    <div className="flex gap-3">
                      {resultContent
                        ? `${resultContent.byteLength} Bytes`
                        : 'Unknown'}

                      {(function () {
                        if (
                          !resultContent ||
                          resultContent.byteLength <= 1024
                        ) {
                          return null;
                        }

                        return (
                          <span className="text-muted-foreground">
                            ({humanFileSize(resultContent.byteLength)})
                          </span>
                        );
                      })()}
                    </div>
                  </div>
                  {resultContent ? (
                    <div>
                      <h4 className="font-bold text-lg mt-1">Contents</h4>
                      {jobResultContent(job, resultContent)}
                    </div>
                  ) : null}
                </div>
              ) : (
                <div>No Result available yet.</div>
              )}
            </CardContent>
          </TabsContent>
        </Tabs>
      </Card>

      <Card className="w-full xl:w-[400px]">
        <CardHeader>
          <CardTitle className="flex justify-between space-x-2 items-center">
            <span>Job Details</span>
            <span
              className={`text-primary dark:text-secondary py-1 px-2 text-base rounded ${colorClassForJob(job)}`}
            >
              {displayJobStatus(job)}
            </span>
          </CardTitle>
        </CardHeader>
        <CardContent className="grow space-y-4">
          <JobProgress job={job} />
          <div>
            <h4 className="font-bold text-lg mt-1">Job ID</h4>
            <p className="overflow-hidden text-ellipsis">{job.id}</p>
          </div>
          <div>
            <h4 className="font-bold text-lg mt-1">Job Name</h4>
            <p className="overflow-hidden text-ellipsis">{job.name}</p>
          </div>
          <div>
            <h4 className="font-bold text-lg mt-1">Dataset ID</h4>
            <p className="overflow-hidden text-ellipsis">{job.datasetId}</p>
          </div>
          <div>
            <h4 className="font-bold text-lg mt-1">Submitted</h4>
            <p className="overflow-hidden text-ellipsis">
              {new Date(job.submitted).toLocaleString()}
            </p>
          </div>
          <div>
            <h4 className="font-bold text-lg mt-1">Progress</h4>
            <div className="flex justify-between">
              <p className="overflow-hidden text-ellipsis">
                {Math.floor(job.progress * 10000) / 100}%
              </p>
              {job.status === IJobStatus.StatusRunning ? (
                <JobStatusIcon job={job}></JobStatusIcon>
              ) : (
                ''
              )}
            </div>
          </div>

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

const getDownloadUrl = (buffer: ArrayBuffer, filetype: string) => {
  let blob = new Blob([buffer], { type: filetype });
  return URL.createObjectURL(blob);
};

const jobResultContent = (job: IJob, resultContent: ArrayBuffer) => {
  const type = job.result?.contentType;
  if (type === IJobResultContentType.ContentTypeI32) {
    // Int

    let view = new DataView(resultContent);
    return <p>{view.getInt32(0, true)}</p>;
  } else if (type == IJobResultContentType.ContentTypeRawBytes) {
    // Binary

    return (
      <a
        href={getDownloadUrl(resultContent, 'application/binary')}
        download={`${job.id}_result.bin`}
        className="underline"
      >
        <Button>Download Result</Button>
      </a>
    );
  } else {
    // Text/JSON
    const isText = type === IJobResultContentType.ContentTypeStringPlain;

    // TODO: json formatting.
    let decoder = new TextDecoder();
    let str = decoder.decode(resultContent);
    return (
      <>
        <div className="my-2 p-2 dark:bg-stone-950 rounded font-mono whitespace-pre">
          {str}
        </div>
        <a
          href={getDownloadUrl(
            resultContent,
            isText ? 'application/text' : 'application/json'
          )}
          download={isText ? 'result.txt' : 'result.json'}
          className="underline"
        >
          Download Result
        </a>
      </>
    );
  }
};
