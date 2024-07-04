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
      setNodes(
        res.data.toSorted((a, b) => {
          if (a.id < b.id) {
            return -1;
          }
          return 1;
        })
      );
    });

    sock.subscribe(topicAllJobs(), (res) => {
      setJobs(res.data);
    });
  }, []);

  return (
    <main className="flex flex-col xl:flex-row p-4 space-y-4 xl:space-y-0 xl:space-x-4">
      <div className="grow grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 min-[1660px]:grid-cols-4 gap-4 grid-flow-row-dense grid-rows auto-rows-[320px]">
        {nodes.map((node) => (
          <Card
            key={node.id}
            className="flex flex-col row-auto"
            style={{
              gridRowEnd: (() => {
                const n = node.workerState.length;
                if (n <= 4) return 'span 1';
                if (n <= 11) return 'span 2';
                if (n <= 18) return 'span 3';
                return 'span 4';
              })(),
            }}
          >
            <CardHeader key={node.id} className="space-y-0 pb-3">
              <CardTitle>
                <span className="text-ellipsis overflow-hidden w-10">
                  {node.info.name}
                </span>
              </CardTitle>

              <CardDescription className="text-muted-foreground pt-2">
                <div className="w-full">
                  <div className="flex flex-row justify-between">
                    <span className="text-nowrap text-xs">
                      <span className="xl:hidden 2xl:inline">Load:</span>
                      {` ${node.workerState.filter((w) => w).length} / ${
                        node.info.numWorkers
                      } worker${node.info.numWorkers === 1 ? '' : 's'}`}
                    </span>

                    <span className="text-nowrap text-xs text-right">
                      <span className="xl:hidden 2xl:inline">IP:</span>
                      {` ${node.info.endpointIP}`}
                    </span>
                  </div>
                  <div className="flex flex-row justify-between">
                    <span className="text-ellipsis overflow-hidden w-full text-nowrap text-xs">
                      ID: {`${node.id}`}
                    </span>

                    <span className="text-nowrap text-xs text-right ml-4">
                      Ping:
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
                        return ` ${duration} ${unit}`;
                      })()}
                    </span>
                  </div>
                </div>
              </CardDescription>
            </CardHeader>
            <CardContent className="p-1 pt-0 h-full overflow-hidden">
              <Separator></Separator>
              <ScrollArea
                className="px-4 py-2 rounded-md"
                style={{
                  overflow: node.workerState.length <= 18 ? 'hidden' : 'auto',
                }}
              >
                {node.workerState.map((workerState, i) => {
                  const job = jobs.find((job) => job.id === workerState);

                  return (
                    <div key={`${i}`}>
                      <div className="text-sm flex items-center space-x-1 w-full">
                        <span className="text-sm text-muted-foreground min-w-4">
                          {i}
                        </span>
                        <WorkerListItem key={i} job={job} workerIndex={i} />
                      </div>
                      {i + 1 < node.workerState.length ? <Separator /> : null}
                    </div>
                  );
                })}
              </ScrollArea>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card className="w-full xl:w-[400px] xl:h-full-pad flex flex-col xl:sticky xl:top-4">
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
