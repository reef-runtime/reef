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
import { BanIcon, CogIcon, } from 'lucide-react';

export default function Home() {
  const { nodes } = useNodes();
  const { jobs } = useJobs();

  return (
    <main className="grid grid-cols-1 md:grid-cols-4 gap-4 p-4">
      {nodes.map((node) => (
        <Card key={node.id} className="flex flex-col">
          <CardHeader key={node.id}>
            <CardTitle>{node.info.name}</CardTitle>
            <CardDescription className='flex flex-col'>
                <span>{`${node.info.endPointIP}`}</span>
                <span>
                {
                    function () {
                        let unit = "min"
                        let duration =  (Date.now() - new Date(node.lastPing).getTime()) / 60000;

                        if (duration < 1) {
                            duration *= 60
                            unit = "sec"
                        }

                        duration = Math.floor(duration)
                        return `${duration} ${unit}`
                    }()
                }
                </span>
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
                        key={`${i}`}
                        className="text-sm flex items-center space-x-1"
                      >
                        <span className='text-sm text-muted-foreground'>{i}</span>

                        <div className='flex justify-center w-5'>
                        {
                            function() {
                                if (job?.status === IJobStatus.StatusStarting) {
                                    return <span className="relative flex h-3 w-3">
                                            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
                                            <span className="relative inline-flex rounded-full h-3 w-3 bg-sky-500"></span>
                                        </span>
                                }

                                if (job?.status === IJobStatus.StatusRunning) {
                                    return <CogIcon className="h-5 w-5 text-orange-400 animate-spin" />
                                }

                                if (job === undefined) {
                                    return <BanIcon className="h-4 w-4 text-gray-300" />
                                }

                                return <span
                                className={classNames('w-3 h-3 rounded-full', {
                                    'bg-gray-400':
                                    job?.status === IJobStatus.StatusQueued,
                                    'bg-green-500':
                                    job?.status === IJobStatus.StatusDone &&
                                    job.result?.success,
                                    'bg-red-500 animate-ping':
                                    job?.status === IJobStatus.StatusDone &&
                                    !job.result?.success,
                                })}
                                ></span>
                            }()
                        }
                        </div>

                        <span className={classNames('text-sm font-medium leading-none',
                            {
                                "text-sm text-muted-foreground": job === undefined
                            },
                        )}>{job?.name ?? 'Worker Idle'}</span>

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
