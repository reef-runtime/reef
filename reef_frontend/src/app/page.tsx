'use client';

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { useNodes } from '@/stores/nodes.store';
// import { useReefStore } from '@/stores/app.store';
// import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useJobs } from '@/stores/job.store';
// import classNames from 'classnames';
import { IJobStatus } from '@/types/job';
// import { BanIcon, CogIcon } from 'lucide-react';
// import JobStatusIcon from '@/components/job-status';
import JobListItem from '@/components/job-list-item';
import WorkerListItem from '@/components/worker-list-item';
import { BanIcon } from 'lucide-react';
import { Separator } from '@/components/ui/separator';
import { useEffect } from 'react';
import { GetSocket, topicNodes, topicAllJobs } from '@/lib/websocket';

export default function Home() {
  const { nodes, setNodes } = useNodes();
  const { jobs, setJobs } = useJobs();

  useEffect(() => {
    const sock = GetSocket();
    sock.unsubscribeAll();

    sock.subscribe(topicNodes(), (res) => {
      setNodes(res.data);
    });

    sock.subscribe(topicAllJobs(), (res) => {
      setJobs(res.data);
    });
  }, []);

  return (
    <main className="flex flex-col xl:flex-row p-4 space-y-4 xl:space-y-0 xl:space-x-4 grow h-full">
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 grow h-full">
        {nodes.map((node) => (
          <Card key={node.id} className="flex flex-col h-[350px]">
            <CardHeader key={node.id} className="space-y-0 pb-3">
              <CardTitle>
                <span className="text-ellipsis overflow-hidden w-10">
                  {node.info.name}
                </span>
              </CardTitle>

              <CardDescription className="text-muted-foreground pt-2">
                <div className="flex justify-between w-full">
                  <div className="w-1/2 mr-4 flex flex-col">
                    <span className="text-nowrap text-xs">{`Load: ${
                      node.workerState.filter((w) => w).length
                    } / ${node.info.numWorkers} worker${
                      node.info.numWorkers === 1 ? '' : 's'
                    }`}</span>
                    <span className="text-ellipsis overflow-hidden w-full text-nowrap text-xs">
                      ID: {`${node.id}`}
                    </span>
                  </div>
                  <Separator orientation="vertical"></Separator>
                  <div className="w-1/2 ml-4 flex flex-col">
                    <span className="text-nowrap text-xs">
                      IP: {`${node.info.endpointIP}`}
                    </span>

                    <span className="text-nowrap text-xs">
                      Ping:{' '}
                      {(function () {
                        let unit = 'min';
                        let duration =
                          (Date.now() - new Date(node.lastPing).getTime()) /
                          60000;

                        if (duration < 1) {
                          duration *= 60;
                          unit = 'sec';
                        }

                        duration = Math.floor(duration);
                        return `${duration} ${unit}`;
                      })()}
                    </span>
                  </div>
                </div>
              </CardDescription>
            </CardHeader>
            <CardContent className="grow min-h-[200px] px-1">
              <Separator></Separator>
              <ScrollArea className="rounded-md grow h-full p-4">
                {node.workerState.map((workerState, i) => {
                  const job = jobs.find((job) => job.id === workerState);

                  return (
                    <div
                      key={`${i}`}
                      className="text-sm flex items-center space-x-1 w-full"
                    >
                      <span className="text-sm text-muted-foreground">{i}</span>
                      <WorkerListItem key={i} job={job} workerIndex={i} />
                    </div>
                  );
                })}
              </ScrollArea>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card className="w-[400px] row-span-full flex flex-col h-full">
        <CardHeader>
          <CardTitle>Completed Jobs</CardTitle>
        </CardHeader>
        <CardContent className="h-full overflow-hidden">
          <ScrollArea className="rounded-md h-full">
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
