'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useNodes } from '@/stores/nodes.store';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useJobs } from '@/stores/job.store';
import { IJob, IJobStatus } from '@/types/job';
import { BanIcon, CogIcon } from 'lucide-react';
import JobListItem from '@/components/job-list-item';
import { useEffect } from 'react';
import { GetSocket, topicAllJobs } from '@/lib/websocket';

const GROUPS = [
  {
    title: 'Queued',
    filter: (job: IJob) => job.status === IJobStatus.StatusQueued,
  },
  {
    title: 'Starting / Running',
    filter: (job: IJob) =>
      job.status === IJobStatus.StatusStarting ||
      job.status === IJobStatus.StatusRunning,
  },
  {
    title: 'Done',
    filter: (job: IJob) => job.status === IJobStatus.StatusDone,
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
    <main className="flex flex-col md:flex-row p-4 md:space-x-4 xl:max-h-dvh h-full">
      <div
        className="flex flex-col xl:grid gap-4 w-full h-full"
        style={{
          gridTemplateColumns: `repeat(${GROUPS.length}, 1fr)`,
        }}
      >
        {GROUPS.map((group) => (
          <Card
            key={group.title}
            className="flex flex-col w-full h-full xl:overflow-hidden"
          >
            <CardHeader key={group.title}>
              <CardTitle>{group.title}</CardTitle>
            </CardHeader>
            <CardContent className="h-full overflow-hidden">
              <ScrollArea className="rounded-md h-full">
                {jobs.filter(group.filter).map((job) => (
                  <JobListItem key={job.id} job={job} />
                ))}
              </ScrollArea>
            </CardContent>
          </Card>
        ))}
      </div>
    </main>
  );
}
