'use client';

import { useEffect } from 'react';
import { useToast } from '@/components/ui/use-toast';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

import init, {
  get_connect_path,
  init_node,
  run_node,
  parse_websocket_data,
  serialize_handshake_response,
  NodeMessageKind,
  NodeMessage,
} from '@/lib/node_web_generated/reef_node_web';

export default function Page() {
  const { toast } = useToast();

  useEffect(() => {
    run();
  }, []);

  return (
    <main className="p-4 h-full w-full">
      <Card className="flex flex-col h-full w-full">
        <CardHeader>
          <CardTitle>Node Web</CardTitle>
        </CardHeader>
        <CardContent className="h-full overflow-hidden">Node web</CardContent>
      </Card>
    </main>
  );
}

let ws: WebSocket | undefined;

async function run() {
  await init();

  if (ws) return;

  let connectPath = get_connect_path();
  console.log(`Connecting to ${connectPath}`);

  ws = new WebSocket(connectPath);
  ws.binaryType = 'arraybuffer';

  await waitWsOpen(ws);
  console.log('Websocket open');

  // do handshake
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

  // node event loop
  while (true) {
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
      let data = new Uint8Array(await res.arrayBuffer());

      init_node(
        message.start_job_data.program_byte_code,
        message.start_job_data.interpreter_state,
        data,
        (log_message: string) => {
          console.log(`Reef log: ${log_message}`);
        },
        (done: number) => {
          console.log(`Reef progress: ${done}`);
        }
      );
    }

    // yield to js event loop
    await sleep(1);
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
