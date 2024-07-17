'use client';

import { Dispatch, Fragment, SetStateAction, useEffect, useState } from 'react';
import { useToast } from '@/components/ui/use-toast';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';

import init, {
  get_connect_path,
  reset_node,
  init_node,
  run_node,
  serialize_state,
  parse_websocket_data,
  serialize_handshake_response,
  serialize_job_state_sync,
  serialize_job_result,
  NodeMessageKind,
  NodeMessage,
} from '@/lib/node_web_generated/reef_node_web';
import { CopyIcon } from 'lucide-react';
import JobProgress from '@/components/job-progress';
import classNames from 'classnames';
import { jobCogSpinner } from '@/components/job-status';
import JobOutput from '@/components/job-output';
import { IJobStatus } from '@/types/job';
import { ILogEntry, ILogKind } from '@/types/log';
import { Separator } from '@/components/ui/separator';

const STATE_SYNC_MILLIS = 1000;

interface NodeState {
  nodeId?: string;
  jobId?: string;
  progress: number;
  logs: ILogEntry[];
}

export default function Page() {
  const { toast } = useToast();

  const [nodeState, setNodeState] = useState<NodeState | undefined>(undefined);

  const closeNode = () => {
    setNodeState(undefined);

    if (ws) {
      ws.close();
      ws = undefined;
    }

    if (wasmInit) reset_node();
  };

  const [url, setUrl] = useState<string>('');

  /* eslint-disable react-hooks/exhaustive-deps */
  useEffect(() => {
    setUrl(window.location.origin);
    return closeNode;
  }, []);
  /* eslint-enable react-hooks/exhaustive-deps */

  function copyPostInstallInstructions() {
    const binary = 'reef_node_native';
    const command = `chmod +x ${binary} \n ./${binary} ${url}`;
    navigator.clipboard.writeText(command);
    toast({
      title: 'Copied',
      description: 'Paste command into terminal to connect.',
    });
  }

  if (!nodeState) {
    return (
      <main className="h-full w-full flex flex-col xl:flex-row p-4 gap-4">
        <Card className="w-full">
          <CardHeader>
            <CardTitle>Join with Node Web</CardTitle>
          </CardHeader>
          <CardContent className="overflow-hidden space-y-4">
            Easily join the Reef Network from your browser with no further
            setup.
            <br />
            <Button
              onClick={() => {
                setNodeState({
                  progress: 0,
                  logs: [],
                });
                runNode(setNodeState);
              }}
            >
              Join now
            </Button>
          </CardContent>
        </Card>

        <Card className="w-full">
          <CardHeader>
            <CardTitle>Join with Node Native</CardTitle>
          </CardHeader>
          <CardContent className="overflow-hidden space-y-2">
            <p>
              Running the Native Reef Node locally allows you to unleash the
              full potential of your hardware and therefore contribute more to
              the Reef network.
            </p>
            <p className="pb-4">
              Note: The Node Native is only available for x86_64 Linux systems.
            </p>

            <p className="text-xl font-semibold">Setup</p>
            <p>Start by downloading the binary for the Node Native.</p>
            <p>
              Navigate to where you downloaded the binary and run these commands
              to execute it.
            </p>

            <div className="flex justify-between font-mono bg-stone-950  p-4 rounded w-full">
              <span className="text-background dark:text-foreground">
                chmod +x ./reef_node_native
                <br />
                ./reef_node_native &quot;{url}&quot;
              </span>

              <Button
                variant="outline"
                size="icon"
                onClick={copyPostInstallInstructions}
              >
                <CopyIcon className="h-4 w-4" />
              </Button>
            </div>

            <div className="h-4"></div>
            <a
              href="/reef_node_native"
              download="reef_node_native"
              className="underline hover:text-gray-400"
            >
              <Button>Download Node Native</Button>
            </a>
          </CardContent>
        </Card>
      </main>
    );
  }

  //
  // Node native is running.
  //

  return (
    <main className="h-full flex flex-col xl:flex-row p-4 gap-4 xl:max-h-dvh">
      <Card className="flex flex-col w-full xl:overflow-hidden">
        <CardHeader>
          <CardTitle>Node Web</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col justify-between">
          <div>
            <div>
              <h3 className="font-bold text-xl">Node ID</h3>
              <p className="overflow-hidden text-ellipsis my-2 p-2 dark:bg-stone-950 rounded font-mono">
                {nodeState.nodeId ?? 'connecting...'}
              </p>
            </div>
            <div>
              <h3 className="font-bold text-xl">Status</h3>
              <p className="overflow-hidden text-ellipsis">
                {nodeState.jobId ? (
                  <div>
                    <div className="flex gap-3 items-center">
                      <span className="text-lg">Executing Job</span>
                      {jobCogSpinner()}
                    </div>

                    <p className="overflow-hidden text-ellipsis my-2 p-2 dark:bg-stone-950 rounded font-mono">
                      {nodeState.jobId}
                    </p>
                  </div>
                ) : (
                  'Idle'
                )}
              </p>
            </div>
            {(function () {
              if (!nodeState.jobId) {
                return null;
              }

              const floored = Math.floor((nodeState.progress * 10000) / 100);

              return (
                <div>
                  <h3 className="font-bold text-xl">Progress</h3>

                  <div className="overflow-hidden text-ellipsis flex flex-col gap-2">
                    <JobProgress
                      progress={nodeState.progress}
                      className="mt-1"
                    />
                    <span className="font-mono font-bold">{floored}%</span>
                  </div>
                </div>
              );
            })()}
          </div>

          <div>
            <Button variant={'destructive'} onClick={closeNode}>
              Disconnect
            </Button>
          </div>
        </CardContent>
      </Card>

      {nodeState.jobId ? (
        <Card className="flex flex-col w-full xl:overflow-hidden">
          <CardHeader>
            <div className="flex justify-between">
              <CardTitle>{nodeState.jobId ? 'Logs' : 'Waiting...'}</CardTitle>
              <span className="font-mono font-bold text-xl overflow-hidden text-ellipsis flex gap-3 items-center">
                <span>Count:</span>
                <span>{nodeState.logs.length}</span>
              </span>
            </div>
          </CardHeader>

          <CardContent className="m-6 mt-0 p-2 dark:bg-stone-950 rounded overflow-hidden">
            {(function () {
              // const logs = nodeState.logs.map((content) => {
              //   return {
              //     kind: ILogKind.LogKindNode,
              //     created: new Date().toISOString(),
              //     content,
              //     jobId: nodeState.jobId,
              //   } as ILogEntry;
              // });

              return (
                <JobOutput
                  compact={false}
                  job={{
                    id: nodeState.jobId,
                    datasetId: '',
                    wasmId: '',
                    name: '',
                    submitted: '',
                    status: IJobStatus.StatusRunning,
                    owner: '',
                    logs: nodeState.logs,
                    progress: nodeState.progress,
                  }}
                ></JobOutput>
              );
            })()}
          </CardContent>
        </Card>
      ) : null}
    </main>
  );
}

let ws: WebSocket | undefined;
let wasmInit = false;

async function runNode(
  setNodeState: Dispatch<SetStateAction<NodeState | undefined>>
) {
  // Initialize Wasm, test for browser features.
  try {
    await init();
    new CompressionStream('gzip');
  } catch (e: any) {
    console.error(e);
    alert(
      'Your browser does not support the features required to run a Reef node'
    );
    location.pathname = '/';
    return;
  }
  wasmInit = true;

  // Start WebSocket.
  if (ws) return;

  let connectPath = get_connect_path();
  console.log(`Connecting to ${connectPath}`);

  ws = new WebSocket(connectPath);
  ws.binaryType = 'arraybuffer';

  await waitWsOpen(ws);
  console.log('Websocket open');

  // Do handshake.
  while (true) {
    let message = await readWsMessage(ws);
    if (message.kind === NodeMessageKind.InitHandShake) break;
  }

  console.log('Starting Handshake...');
  // TODO: userAgent stuff for better name
  ws.send(serialize_handshake_response(1, 'node@web'));

  let nodeId;
  while (true) {
    let message = await readWsMessage(ws);
    if (message.kind === NodeMessageKind.AssignId) {
      if (!message.assign_id_data) throw 'message invariant violation';
      nodeId = message.assign_id_data;
      break;
    }
  }

  console.log(
    `%c==> Handshake successful: node ${nodeId} is connected.`,
    'font-weight:bold;'
  );

  // queue messages for reading if they come in while something else is running
  let messageQueue: NodeMessage[] = [];
  ws.addEventListener('message', (event: any) => {
    let array = new Uint8Array(event.data);
    try {
      let message = parse_websocket_data(array);
      messageQueue.push(message);
    } catch (e: any) {
      console.error('Error Reading WebSocket:', e);
    }
  });

  let internalState = {
    nodeId,
    jobId: undefined as string | undefined,
    progress: 0,
    logs: [] as ILogEntry[],
    logsFlush: [] as string[],
    lastSync: 0,
  };

  const updateUi = () => {
    setNodeState({
      nodeId: internalState.nodeId,
      jobId: internalState.jobId,
      progress: internalState.progress,
      logs: internalState.logs,
    });
  };
  updateUi();

  const reset = () => {
    reset_node();
    internalState.jobId = undefined;
    internalState.progress = 0;
    internalState.logs = [];
    internalState.logsFlush = [];
    internalState.lastSync = 0;
  };
  const enc = new TextEncoder();

  // node event loop
  while (true) {
    // if the WebSocket is no longer set we should exit
    if (!ws) break;

    let errorMessage: string | undefined;

    // read message from WebSocket
    let message = messageQueue.shift();
    if (message) {
      // we only care about start/abort job commands
      if (message.kind === NodeMessageKind.StartJob) {
        if (!message.start_job_data) throw 'message invariant violation';
        if (internalState.jobId)
          throw 'attempted to start job while another one is still running';

        console.log(
          `%c==> Starting job ${message.start_job_data.job_id}.`,
          'font-weight:bold;'
        );

        // fetch dataset
        let res = await fetch(
          `/api/dataset/${message.start_job_data.dataset_id}`
        );
        let dataset = new Uint8Array(await res.arrayBuffer());

        try {
          init_node(
            message.start_job_data.program_byte_code,
            message.start_job_data.interpreter_state,
            dataset,
            (log_message: string) => {
              internalState.logs.push({
                kind: ILogKind.LogKindProgram,
                created: new Date().toISOString(),
                jobId: internalState.jobId ? internalState.jobId : '',
                content: log_message,
              });
              internalState.logsFlush.push(log_message);
            },
            (done: number) => {
              internalState.progress = done;
            }
          );

          if (
            message.start_job_data.interpreter_state &&
            message.start_job_data.interpreter_state.length > 0
          ) {
            internalState.logs.push({
              kind: ILogKind.LogKindNode,
              created: new Date().toISOString(),
              jobId: '',
              content: `Resuming previously executed job with ${Math.floor(message.start_job_data.progress * 100)}% progress...`,
            });
          }

          internalState.jobId = message.start_job_data.job_id;
        } catch (e: any) {
          console.log('Error starting:', e);
          errorMessage = e;
        }

        updateUi();
      } else if (message.kind === NodeMessageKind.AbortJob) {
        if (!message.abort_job_data) throw 'message invariant violation';
        if (!internalState.jobId)
          throw 'attempted to abort job while none is running';

        if (internalState.jobId !== message.abort_job_data) {
          throw 'attempted to abort job that is not running on this node';
        }

        errorMessage = 'Job aborted';
      }
    }

    let sleepDuration = 0.01;

    // Only perform if job is running
    if (internalState.jobId) {
      // State sync
      if (internalState.lastSync + STATE_SYNC_MILLIS < Date.now()) {
        let interpreterState = serialize_state();

        const stream = new Blob([interpreterState])
          .stream()
          .pipeThrough(new CompressionStream('gzip'));
        const chunks = [];
        // @ts-ignore
        for await (const chunk of stream) {
          chunks.push(chunk);
        }
        const compressedState = new Uint8Array(
          await new Blob(chunks).arrayBuffer()
        );

        console.log(
          `Serialized ${compressedState.length} bytes for state sync.`
        );

        ws.send(
          serialize_job_state_sync(
            internalState.progress,
            compressedState,
            internalState.logsFlush
          )
        );

        internalState.logsFlush = [];

        internalState.lastSync = Date.now();
      }

      let result;
      try {
        // TODO: benchmark what works best
        result = run_node(0x10000);

        if (result.done) {
          if (!result.job_output) throw 'message invariant violation';

          // final state sync
          ws.send(
            serialize_job_state_sync(
              1,
              new Uint8Array(),
              internalState.logsFlush
            )
          );

          // verify content type
          if (
            result.job_output.content_type < 0 ||
            result.job_output.content_type > 3
          ) {
            errorMessage = 'Invalid job output content type';
            throw 'invalid content type';
          }
          ws.send(
            serialize_job_result(
              true,
              result.job_output.data,
              result.job_output.content_type
            )
          );

          console.log(
            `%c==> Job ${internalState.jobId} has has executed successfully.`,
            'font-weight:bold;'
          );

          reset();
        } else {
          sleepDuration = result.sleep_for ?? 0;
        }
      } catch (e: any) {
        console.log('Error executing:', e);
        errorMessage = errorMessage ?? e;
      }

      if (errorMessage) {
        ws.send(serialize_job_result(false, enc.encode(errorMessage), 2));

        console.log(
          `%c==> Job ${internalState.jobId} has has failed.`,
          'font-weight:bold;'
        );

        reset();
      }
    }

    updateUi();

    // yield to js event loop
    await sleep(sleepDuration * 1000);
  }
}

const waitWsOpen = async (ws: WebSocket): Promise<void> => {
  return new Promise((res) => {
    const openHandler = () => {
      ws.removeEventListener('open', openHandler);
      res();
    };
    ws.addEventListener('open', openHandler);
  });
};

const readWsMessage = async (ws: WebSocket): Promise<NodeMessage> => {
  return new Promise((res) => {
    const messageHandler = (event: any) => {
      ws.removeEventListener('message', messageHandler);

      let array = new Uint8Array(event.data);
      try {
        let message = parse_websocket_data(array);
        res(message);
      } catch (e: any) {
        console.error('Error Reading WebSocket:', e);
      }
    };
    ws.addEventListener('message', messageHandler);
  });
};

const sleep = async (ms: number): Promise<void> => {
  return new Promise((res) => {
    setTimeout(res, ms);
  });
};
