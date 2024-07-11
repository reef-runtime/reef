'use client';

import { Dispatch, SetStateAction, useEffect, useState } from 'react';
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

const STATE_SYNC_MILLIS = 5000;

interface NodeState {
  nodeId?: string;
  jobId?: string;
  progress: number;
  logs: string[];
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

  if (!nodeState) {
    return (
      <main className="h-full w-full flex flex-col xl:flex-row p-4 gap-4">
        <Card className="h-full w-full">
          <CardHeader>
            <CardTitle>Join with Node Web</CardTitle>
          </CardHeader>
          <CardContent className="h-full overflow-hidden space-y-4">
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

        <Card className="h-full w-full">
          <CardHeader>
            <CardTitle>Join with Node Native</CardTitle>
          </CardHeader>
          <CardContent className="h-full overflow-hidden space-y-2">
            <p>
              Running the Native Reef Node locally allows you to unleash the
              full potential of your hardware and therfore contribute more ot
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
            <div className="font-mono bg-stone-950 text-slate-50 p-4 rounded">
              chmod +x ./reef_node_native
              <br />
              ./reef_node_native "{url}"
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

  return (
    <main className="p-4 h-full w-full">
      <Card className="h-full w-full">
        <CardHeader>
          <CardTitle>Node Web</CardTitle>
        </CardHeader>
        <CardContent className="h-full overflow-hidden space-y-4">
          <div>
            <h4 className="font-bold">Node ID</h4>
            <p className="overflow-hidden text-ellipsis">
              {nodeState.nodeId ?? 'connecting...'}
            </p>
          </div>
          <div>
            <h4 className="font-bold">Job ID</h4>
            <p className="overflow-hidden text-ellipsis">
              {nodeState.jobId ?? 'None'}
            </p>
          </div>
          <div>
            <h4 className="font-bold">Progress</h4>
            <p className="overflow-hidden text-ellipsis">
              {nodeState.progress}
            </p>
          </div>
          <div>
            <h4 className="font-bold">Log count</h4>
            <p className="overflow-hidden text-ellipsis">
              {nodeState.logs.length}
            </p>
          </div>

          <Button variant={'destructive'} onClick={closeNode}>
            Disconnect
          </Button>
        </CardContent>
      </Card>
    </main>
  );
}

let ws: WebSocket | undefined;
let wasmInit = false;

async function runNode(
  setNodeState: Dispatch<SetStateAction<NodeState | undefined>>
) {
  // Initialize Wasm
  try {
    await init();
    new CompressionStream('gzip');
  } catch (e: any) {
    console.error(e);
    alert(
      'Your browser does not support the features required to run a Reef node!'
    );
    location.pathname = '/';
    return;
  }
  wasmInit = true;

  // Start WebSocket
  if (ws) return;

  let connectPath = get_connect_path();
  console.log(`Connecting to ${connectPath}`);

  ws = new WebSocket(connectPath);
  ws.binaryType = 'arraybuffer';

  await waitWsOpen(ws);
  console.log('Websocket open');

  // Do handshake
  while (true) {
    let message = await readWsMessage(ws);
    if (message.kind === NodeMessageKind.InitHandShake) break;
  }

  console.log('Starting Handshake');
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
    logs: [] as string[],
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
      // we only care about start job commands
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
              internalState.logs.push(log_message);
              internalState.logsFlush.push(log_message);
            },
            (done: number) => {
              internalState.progress = done;
            }
          );

          internalState.jobId = message.start_job_data.job_id;
        } catch (e: any) {
          console.log('Error starting:', e);
          errorMessage = e;
        }

        updateUi();
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
        errorMessage = e;
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
