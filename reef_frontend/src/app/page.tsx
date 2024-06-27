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
import { BanIcon, CogIcon } from 'lucide-react';
import JobStatusIcon from '@/components/job-status';
import JobListItem from '@/components/job-list-item';

export default function Home() {
  const { nodes } = useNodes();
  const { jobs } = useJobs();

  return (
    <main className="flex flex-col xl:flex-row p-4 space-y-4 xl:space-y-0 xl:space-x-4 grow">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 grow">
        {nodes.map((node) => (
          <Card key={node.id} className="flex flex-col">
            <CardHeader key={node.id}>
              <CardTitle>{node.info.name}</CardTitle>
              <CardDescription className="flex flex-col">
                <span>{`${node.info.endPointIP}`}</span>
                <span>
                  {(function () {
                    let unit = 'min';
                    let duration =
                      (Date.now() - new Date(node.lastPing).getTime()) / 60000;

                    if (duration < 1) {
                      duration *= 60;
                      unit = 'sec';
                    }

                    duration = Math.floor(duration);
                    return `${duration} ${unit}`;
                  })()}
                </span>
              </CardDescription>
            </CardHeader>
            <CardContent className="grow min-h-[200px]">
              <ScrollArea className="rounded-md grow h-full">
                <h4 className="mb-4 text-sm font-medium leading-none">{`${node.info.numWorkers} workers`}</h4>
                {node.workerState.map((workerState, i) => {
                  const job = jobs.find((job) => job.id === workerState);

                  return (
                    <>
                      <div
                        key={`${i}`}
                        className="text-sm flex items-center space-x-1"
                      >
                        <span className="text-sm text-muted-foreground">
                          {i}
                        </span>

                        <JobStatusIcon job={job} />
                        <span
                          className={classNames(
                            'text-sm font-medium leading-none',
                            {
                              'text-sm text-muted-foreground':
                                job === undefined,
                            }
                          )}
                        >
                          {job?.name ?? 'Worker Idle'}
                        </span>
                      </div>
                      <Separator className="my-2" />
                    </>
                  );
                })}
              </ScrollArea>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card className="w-[400px] row-span-full flex flex-col">
        <CardHeader>
          <CardTitle>Completed Jobs</CardTitle>
        </CardHeader>
        <CardContent>
          <ScrollArea className="rounded-md grow">
            {jobs
              .filter((job) => job.status === IJobStatus.StatusDone)
              .map((job) => (
                <JobListItem key={job.id} job={job} />
              ))}
          </ScrollArea>
        </CardContent>
      </Card>
    </main>
  );
}
