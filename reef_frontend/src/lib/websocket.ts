import { IJob } from '@/types/job';
import { INode } from '@/types/node';

//
// Topics.
//

export enum TopicKind {
  Nodes = 'nodes',
  SingleJob = 'single_job',
  AllJobs = 'all_jobs',
}

export interface Topic<T extends TopicKind = TopicKind> {
  kind: T;
  additional?: string;
}

export function topicAllJobs(): Topic<TopicKind.AllJobs> {
  return { kind: TopicKind.AllJobs };
}

export function topicSingleJob(id: string): Topic<TopicKind.SingleJob> {
  return { kind: TopicKind.SingleJob, additional: id };
}

export function topicNodes(): Topic<TopicKind.Nodes> {
  return { kind: TopicKind.Nodes };
}

export type UpdateMessage<T> = T extends TopicKind.Nodes
  ? { topic: Topic<T>; data: INode[] }
  : T extends TopicKind.SingleJob
    ? { topic: Topic<T>; data: IJob }
    : T extends TopicKind.AllJobs
      ? { topic: Topic<T>; data: IJob[] }
      : never;

type OnMessageCallBack<T extends TopicKind> = (data: UpdateMessage<T>) => void;

//
// Websocket.
//

export type SocketCallback = () => void;

export const sleep = (ms: number) => new Promise((res) => setTimeout(res, ms));

export class ReefWebsocket {
  socket: WebSocket;
  callbacks: Map<string, any>;
  isReady: boolean = false;

  constructor() {
    this.callbacks = new Map();

    let protocol = undefined;
    const host = document.location.host;

    switch (document.location.protocol) {
      case 'http:':
        protocol = 'ws:';
        break;
      case 'https:':
        protocol = 'wss:';
        break;
      default:
        throw `Unsupported protocol '${document.location.protocol}':
                        only http and https are supported`;
    }

    let url = `${protocol}//${host}/api/updates`;

    this.socket = new WebSocket(url);

    this.socket.onopen = () => {
      this.isReady = true;
      this.sync();
    };

    this.socket.onclose = () => {
      throw 'Websocket closed prematurely';
    };

    this.socket.onmessage = (evt) => {
      let payload = JSON.parse(evt.data) as UpdateMessage<TopicKind>;

      if (!payload.topic.additional) {
        delete payload.topic.additional;
      }

      this.onMessage(payload);
    };
  }

  private onMessage(data: UpdateMessage<TopicKind>) {
    const callback = this.callbacks.get(JSON.stringify(data.topic));
    if (!callback) {
      throw `Required callback does not exist for topic ${data.topic.kind} (filter=${data.topic.additional})`;
    }

    callback(data);
  }

  private sync() {
    if (!this.isReady) {
      console.log('Socket not ready, waiting...');
      return;
    }

    let topics: string[] = Array.from(this.callbacks.keys());

    let topicUn = topics.map((u) => JSON.parse(u));

    this.socket.send(
      JSON.stringify({
        topics: topicUn,
      })
    );
    console.log('Sent websocket sync.');
  }

  subscribe<K extends TopicKind>(
    topic: Topic<K>,
    callback: OnMessageCallBack<K>
  ) {
    console.dir(this);
    this.callbacks.set(JSON.stringify(topic), callback);
    this.sync();
  }

  unsubscribeAll() {
    this.callbacks.clear();
    this.sync();
  }
}

//
// Websocket singleton.
//

let sock: ReefWebsocket | undefined = undefined;

export function GetSocket() {
  if (!sock) {
    sock = new ReefWebsocket();
  }
  return sock;
}
