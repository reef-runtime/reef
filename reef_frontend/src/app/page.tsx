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
import { IJobStatus } from '@/types/job';

export default function Home() {
  const { nodes } = useNodes();
  const { jobs } = useJobs();

  return (
    <main className="grid grid-cols-1 md:grid-cols-4 gap-4 p-4">
      {nodes.map((node) => (
        <Card key={node.id} className="flex flex-col">
          <CardHeader>
            <CardTitle>{node.info.name}</CardTitle>
            <CardDescription>
              <ol>
                <li>{`${node.info.endPointIP}`}</li>
                <li>
                  {`Last Ping: ${
                    Math.floor(
                      (Date.now() - new Date(node.lastPing).getTime()) / 60000
                    ) + ' min'
                  }`}
                </li>
              </ol>
            </CardDescription>
          </CardHeader>
          <CardContent className="grow">
            <ScrollArea className="rounded-md border grow h-full">
              <div className="p-4">
                <h4 className="mb-4 text-sm font-medium leading-none">{`${node.info.numWorkers} workers`}</h4>
                {node.workerState.map((workerState, i) => {
                  const job = jobs.find((job) => job.id === workerState);

                  return (
                    <>
                      <div
                        key={i}
                        className="text-sm flex items-center space-x-1"
                      >
                        <span
                          className={classNames('w-3 h-3 rounded-full', {
                            'bg-gray-400 animate-pulse':
                              job?.status === IJobStatus.StatusQueued,
                            'bg-yellow-500 animate-pulse':
                              job?.status === IJobStatus.StatusRunning,
                            'bg-green-500':
                              job?.status === IJobStatus.StatusDone &&
                              job.result?.success,
                            'bg-red-500':
                              job?.status === IJobStatus.StatusDone &&
                              !job.result?.success,
                          })}
                        ></span>
                        <span>{job?.name ?? 'Idle'}</span>
                      </div>
                      <Separator className="my-2" />
                    </>
                  );
                })}
              </div>
            </ScrollArea>
          </CardContent>
        </Card>
      ))}
    </main>
  );
}
