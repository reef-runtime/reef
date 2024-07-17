'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useNodes } from '@/stores/nodes.store';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useJobs } from '@/stores/job.store';
import { IJob, IJobStatus } from '@/types/job';
import { BanIcon, CogIcon } from 'lucide-react';
import JobListItem, {
  JobListItemPlaceholder,
} from '@/components/job-list-item';
import { useEffect } from 'react';
import { GetSocket, topicAllJobs } from '@/lib/websocket';

const GROUPS = [
  {
    title: 'Queued',
    filter: (job: IJob) => job.status === IJobStatus.StatusQueued,
    states: [IJobStatus.StatusQueued]
  },
  {
    title: 'Starting / Running',
    filter: (job: IJob) =>
      job.status === IJobStatus.StatusStarting ||
      job.status === IJobStatus.StatusRunning,
    states: [IJobStatus.StatusStarting, IJobStatus.StatusRunning]
  },
  {
    title: 'Done',
    filter: (job: IJob) => job.status === IJobStatus.StatusDone,
    states: [IJobStatus.StatusDone]
  },
];

export default function Page() {
  const { jobs, setJobs } = useJobs();

  /* eslint-disable react-hooks/exhaustive-deps */
  useEffect(() => {
    const sock = GetSocket();
    sock.unsubscribeAll();

    sock.subscribe(topicAllJobs(), (res) => {
      setJobs(res.data);
    });
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  return (
    <main className="flex flex-col xl:flex-row p-4 gap-4 xl:max-h-dvh h-full">
      {GROUPS.map((group) => (
        <Card
          key={group.title}
          className="flex flex-col w-full xl:overflow-hidden"
        >
          <CardHeader key={group.title}>
            <CardTitle>{group.title}</CardTitle>
          </CardHeader>
          <CardContent className="overflow-hidden">
            {(() => {
              const isDoneGroup = group.states.includes(IJobStatus.StatusDone)
              const sortFunc = isDoneGroup ? (a: IJob, b: IJob) => {
                const dateA = new Date(a.result!.created).getTime()
                const dateB = new Date(b.result!.created).getTime()
                return dateA > dateB ? -1 : 1
              } : (a: IJob, b: IJob) => {
                const dateA = new Date(a.submitted).getTime()
                const dateB = new Date(b.submitted).getTime()
                return dateA > dateB ? -1 : 1
              }

              const groupJobs = jobs.filter(group.filter).sort(sortFunc);
              if (groupJobs.length > 0) {
                return (
                  <ScrollArea className="rounded-md">
                    {groupJobs.map((job) => (
                      <JobListItem key={job.id} job={job} />
                    ))}
                  </ScrollArea>
                );
              } else {
                return <JobListItemPlaceholder />;
              }
            })()}
          </CardContent>
        </Card>
      ))}
    </main>
  );
}
