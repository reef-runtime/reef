'use client';

import { useEffect, useState } from 'react';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import { AppWindowMac, Terminal } from 'lucide-react';

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Separator } from '@/components/ui/separator';
import { useToast } from '@/components/ui/use-toast';
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form';

import JobListItem, {
  JobListItemPlaceholder,
} from '@/components/job-list-item';
import WorkerListItem from '@/components/worker-list-item';

import { useNodes } from '@/stores/nodes.store';
import { useJobs } from '@/stores/job.store';
import { IJob, IJobStatus } from '@/types/job';
import { GetSocket, topicNodes, topicAllJobs } from '@/lib/websocket';
import { ReefSession, useReefSession } from '@/stores/session.store';

export default function Home() {
  const { nodes, setNodes } = useNodes();
  const { jobs, setJobs } = useJobs();

  /* eslint-disable react-hooks/exhaustive-deps */
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
  /* eslint-enable react-hooks/exhaustive-deps */

  // async function fetchAuth(token: string | null) {
  //   const auth = await (
  //     await fetch('/api/auth', {
  //       method: 'POST',
  //       body: JSON.stringify({
  //         token,
  //       }),
  //     })
  //   ).json();
  //
  //   console.log(auth);
  // }

  const { session, fetchSession } = useReefSession();

  const { toast } = useToast();

  async function loginHandler(token: string | null) {
    try {
      const sess: ReefSession = await fetchSession(token);

      if (token) {
        if (!sess.isAdmin) {
          toast({
            title: 'Login Failed',
            description: 'Backend failure: no admin rights granted',
          });
        } else {
          toast({
            title: 'Login Success',
            description: `New admin session id: ${sess.id}`,
          });
        }
      }
    } catch (e: any) {
      toast({
        title: 'Login Failed',
        description: e,
      });
    }
  }

  useEffect(() => {
    loginHandler(null);
  }, []);

  const [isDialogOpen, setIsDialogOpen] = useState(false);

  useEffect(() => {
    window.addEventListener('keypress', (event: KeyboardEvent) => {
      const { key } = event;
      if (key === 'q') {
        event.preventDefault();
        setIsDialogOpen(true);
      }
    });
  }, []);

  const schema = z.object({
    token: z.string().min(1),
  });

  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      token: '',
    },
  });

  function onSubmit(values: z.infer<typeof schema>) {
    setIsDialogOpen(false);
    loginHandler(values.token);
  }

  const [time, setTime] = useState(Date.now());

  useEffect(() => {
    const interval = setInterval(() => setTime(Date.now()), 1000);

    return () => {
      clearInterval(interval);
    };
  }, []);

  return (
    <main className="flex flex-col xl:flex-row p-4 space-y-4 xl:space-y-0 xl:space-x-4">
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Enter Administrator Mode</DialogTitle>
            <DialogDescription>
              Once activated, all jobs can be aborted and killed.
            </DialogDescription>
          </DialogHeader>
          <div className="flex items-center space-x-2">
            <Form {...form}>
              <form
                onSubmit={form.handleSubmit(onSubmit)}
                className="space-y-8"
              >
                <FormField
                  control={form.control}
                  name="token"
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Admin Token</FormLabel>
                      <FormControl>
                        <Input type="password" {...field} />
                      </FormControl>
                      <FormDescription>
                        Set by the <code>REEF_ADMIN_TOKEN</code> environment
                        variable.
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </form>
            </Form>
          </div>
          <DialogFooter className="sm:justify-start">
            <Button
              type="button"
              variant="secondary"
              onClick={() => {
                setIsDialogOpen(false);
                loginHandler(form.getValues().token);
              }}
            >
              Login
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <div className="grow">
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 min-[1780px]:grid-cols-4 gap-4 grid-flow-row-dense grid-rows auto-rows-[320px]">
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
                            (Math.abs(Date.now() - new Date(node.lastPing).getTime())) /
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
              <CardContent className="p-1 pt-0 grow overflow-hidden">
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

        {nodes.length === 0 ? (
          <div className="h-full min-h-[250px] flex flex-col place-items-center justify-center space-y-2 text-muted-foreground">
            <AppWindowMac size={60} strokeWidth={1.5} />
            <div className="text-xl ">No Nodes connected</div>
          </div>
        ) : null}
      </div>
      <Card className="min-h-[250px] xl:w-[400px] xl:h-full-pad flex flex-col xl:sticky xl:top-4">
        <CardHeader>
          <CardTitle>Completed Jobs</CardTitle>
        </CardHeader>
        <CardContent className="h-full overflow-hidden">
          {(() => {
            const groupJobs = jobs.filter(
              (job) => job.status === IJobStatus.StatusDone
            ).sort((a: IJob, b: IJob) => {
                    if (!a.result || !b.result) {
                        return 1
                    }
                    const dateA = new Date(a.result!.created).getTime();
                    const dateB = new Date(b.result!.created).getTime();
                    return dateA > dateB ? -1 : 1;
            });
            if (groupJobs.length > 0) {
              return (
                <ScrollArea className="rounded-md h-full">
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
    </main>
  );
}
