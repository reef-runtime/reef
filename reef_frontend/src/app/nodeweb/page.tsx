'use client';

import { Dispatch, SetStateAction, useEffect, useState } from 'react';
import { useToast } from '@/components/ui/use-toast';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

import init, {
  get_connect_path,
  reset_node,
  init_node,
  run_node,
  parse_websocket_data,
  serialize_handshake_response,
  NodeMessageKind,
  NodeMessage,
} from '@/lib/node_web_generated/reef_node_web';

interface NodeState {
  nodeId?: string;
  jobId?: string;
  progress: number;
  logs: string[];
}

export default function Page() {
  const { toast } = useToast();

  const [nodeState, setNodeState] = useState<NodeState>({
    progress: 0,
    logs: [],
  });

  // retarted react hack to not have dev mount+unmount fuck up our logic
  let mountTimeout: NodeJS.Timeout | undefined;
  useEffect(() => {
    mountTimeout = setTimeout(() => {
      run(setNodeState);
    }, 200);

    return () => {
      if (mountTimeout) {
        clearTimeout(mountTimeout);
        mountTimeout = undefined;
      }

      if (ws) {
        ws.close();
        ws = undefined;
      }

      if (wasmInit) reset_node();
    };
  }, []);

  return (
    <main className="p-4 h-full w-full">
      <Card className="flex flex-col h-full w-full">
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
        </CardContent>
      </Card>
    </main>
  );
}

let ws: WebSocket | undefined;
let wasmInit = false;

async function run(setNodeState: Dispatch<SetStateAction<NodeState>>) {
  // Initialize Wasm
  await init();
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

  let interalState = {
    nodeId,
    jobId: undefined,
    progress: 0,
    logs: [],
  };

  const updateUi = () => {
    setNodeState({
      nodeId: interalState.nodeId,
      jobId: interalState.jobId,
      progress: interalState.progress,
      logs: interalState.logs,
    });
  };
  updateUi();

  // node event loop
  while (true) {
    // if the websocket is no longer set we should exit
    if (!ws) break;

    // read message
    let message = messageQueue.shift();
    if (message) {
      // we only care about start job commands
      if (message.kind === NodeMessageKind.StartJob) {
        if (!message.start_job_data) throw 'message invariant violation';
      }

      if (!message.start_job_data) throw 'message invariant violation';

      // fetch dataset
      let res = await fetch(
        `/api/dataset/${message.start_job_data.dataset_id}`
      );
      let dataset = new Uint8Array(await res.arrayBuffer());

      console.log(message.start_job_data.program_byte_code);

      init_node(
        message.start_job_data.program_byte_code,
        message.start_job_data.interpreter_state,
        dataset,
        (log_message: string) => {
          console.log(`Reef log: ${log_message}`);
        },
        (done: number) => {
          console.log(`Reef progress: ${done}`);
        }
      );

      updateUi();
    }

    // yield to js event loop
    await sleep(0);
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
