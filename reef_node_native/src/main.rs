use std::sync::atomic::Ordering;
use std::sync::Mutex;
use std::thread;
use std::time::{Duration, Instant};
use std::{
    net::TcpStream,
    sync::{
        atomic::AtomicU8,
        mpsc::{self, TryRecvError},
        Arc,
    },
};

use anyhow::{bail, Context, Result};
use capnp::{message::ReaderOptions, serialize};
use clap::Parser;
use tungstenite::{stream::MaybeTlsStream, Message, WebSocket};
use url::Url;

use reef_protocol_node::message_capnp::{
    message_to_node::{self, body},
    MessageToNodeKind,
};

mod comms;
mod handshake;
mod worker;
use worker::{FromWorkerMessage, Job};

type WSConn = WebSocket<MaybeTlsStream<TcpStream>>;

use crate::worker::{spawn_worker_thread, WorkerSignal};

const NODE_REGISTER_PATH: &str = "/api/node/connect";

const MAIN_THREAD_SLEEP: Duration = Duration::from_millis(10);

/// Reef worker node (native)
#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    #[arg(short, long)]
    // Name to be sent to the manager (default is the hostname + extra infos)
    node_name: Option<String>,

    #[arg(short = 'u', long)]
    // Base url of the manager.
    manager_url: Url,

    #[arg(short = 's', long)]
    // Whether to use https to connect to the manager.
    tls: bool,

    #[arg(short = 'm', long)]
    // How many milliseconds to wait before syncs.
    sync_delay_millis: usize,
}

struct NodeState(Vec<Job>);

impl NodeState {
    fn new(num_workers: usize) -> Self {
        Self(Vec::with_capacity(num_workers))
    }
}

fn main() -> anyhow::Result<()> {
    let args = Args::parse();

    //
    // Create connection.
    //
    let scheme = match args.tls {
        true => "wss",
        false => "ws",
    };

    let mut connect_url = args.manager_url.clone();
    connect_url.set_path(NODE_REGISTER_PATH);
    connect_url.set_scheme(scheme).unwrap();

    println!("Connecting to {}...", &connect_url);

    env_logger::init();

    let (mut socket, response) = tungstenite::connect(connect_url).expect("Can't connect");

    println!("Connected to the manager");
    println!("Registration response HTTP code: {}", response.status());

    //
    // Perform handshake.
    //
    let num_workers = std::thread::available_parallelism().map(|n| n.get()).unwrap_or(1);

    let node_name = match args.node_name {
        Some(from_args) => from_args,
        None => {
            let hostname = sysinfo::System::host_name().with_context(|| "failed to determine system hostname")?;
            format!("native@{hostname}")
        }
    };

    let mut state = NodeState::new(num_workers);

    let node_info =
        handshake::perform(&node_name, num_workers as u16, &mut socket).with_context(|| "handshake failed")?;

    println!("===> Handshake successful: node {} is connected.", hex::encode(node_info.node_id));

    let sync_wait_duration = Duration::from_millis(args.sync_delay_millis as u64);

    loop {
        //
        // Websocket communications.
        //
        if socket.can_read() {
            let msg = socket.read().expect("Error reading message");
            state.handle_websocket(msg).with_context(|| "evaluating incoming message")?;
        }

        //
        // Worker channels communications.
        //
        let mut finished_worker_indices = vec![];
        for worker in state.0.iter_mut() {
            match worker.channel_from_worker.try_recv() {
                Ok(FromWorkerMessage::Log(log)) => {
                    println!("recv log: {}:{} {}", log.content, log.kind, worker.worker_index);

                    worker.logs_to_be_flushed.push(log);
                }
                Ok(FromWorkerMessage::Progress(new)) => {
                    worker.progress = new;
                }
                Ok(FromWorkerMessage::Sleep(seconds)) => {
                    let sleep_until = &mut worker.sleep_until.lock().unwrap();
                    **sleep_until =
                        Some(Instant::now().checked_add(Duration::from_millis((seconds * 1000f32) as u64)).unwrap());
                    worker.signal_to_worker.store(WorkerSignal::SLEEP, Ordering::Relaxed);
                }
                Ok(FromWorkerMessage::State(interpreter_state)) => {
                    worker.flush_state(&interpreter_state, &mut socket)?;
                }
                Err(TryRecvError::Empty) => thread::sleep(MAIN_THREAD_SLEEP),
                Err(TryRecvError::Disconnected) => unreachable!("all senders have been dropped"),
            }

            //
            // Checking whether a thread has finished.
            //
            if worker.handle.is_finished() {
                finished_worker_indices.push(worker.worker_index);
            }

            //
            // Send a sync request if enough time has passed.
            //
            if worker.last_sync.duration_since(Instant::now()) >= sync_wait_duration {
                worker.signal_to_worker.store(WorkerSignal::SAVE_STATE, Ordering::Relaxed);
            }
        }

        //
        // Remove all finished jobs.
        //
        for worker_idx in finished_worker_indices {
            let idx_in_vec = state.0.iter().position(|w| w.worker_index == worker_idx).unwrap();

            let mut worker = state.0.remove(idx_in_vec);

            // Transfer any logs and the final progress reading to the manager.
            // State can be empty since it is not required anymore.
            worker.flush_state(&[], &mut socket)?;

            let res = worker.handle.join().expect("worker thread panic'ed, this is a bug");

            match res {
                Ok(ret_val) => println!("Job has executed successfully! {}", ret_val.0),
                Err(err) => println!("Job failed: {err:?}"),
            }
        }
    }

    // TODO: implement a close (also on reef protocol layer)
    // socket.close(None);
}

impl NodeState {
    fn worker_exists(&self, worker_index: usize) -> bool {
        self.0.iter().any(|w| w.worker_index == worker_index)
    }

    fn handle_websocket(&mut self, msg: tungstenite::Message) -> Result<()> {
        let action = match msg {
            Message::Text(_) => bail!("received a text message, this should never happen"),
            Message::Binary(bin) => handle_binary(&bin)?,
            Message::Ping(data) => {
                if !data.is_empty() {
                    println!("[warning] ping data is not empty: {data:?}")
                }
                Action::Ping
            }
            Message::Pong(_) => Action::Pong,
            Message::Close(_) => Action::Disconnect,
            Message::Frame(_) => unreachable!("received a raw frame, this should never happen"),
        };

        match action {
            Action::StartJob(request) => {
                if let Err(err) = self.start_job(request) {
                    // TODO: replace with `real` logger.
                    eprintln!("Failed to start job: {err}");
                }
            }
            Action::Ping => {
                println!("received ping, would send pong here...");
            }
            Action::Pong => todo!(),
            Action::Disconnect => todo!(),
        }

        Ok(())
    }

    fn start_job(&mut self, request: StartJobRequest) -> Result<()> {
        // 1. Check if the worker is free and available.
        if self.worker_exists(request.worker_index) {
            bail!("requested illegal worker index");
        }

        println!(
            "starting job with id {:?} on worker {} with program: {:?}...",
            request.job_id, request.worker_index, request.program_byte_code
        );

        let signal = Arc::new(AtomicU8::new(0));

        let (to_master_sender, from_worker_receiver) = mpsc::channel();

        let sleep_until = Arc::new(Mutex::new(None));

        let handle = spawn_worker_thread(
            to_master_sender,
            signal.clone(),
            request.program_byte_code,
            request.job_id.clone(),
            sleep_until.clone(),
        );

        let worker = Job {
            worker_index: request.worker_index,
            job_id: request.job_id,
            signal_to_worker: signal.clone(),
            channel_from_worker: from_worker_receiver,
            handle,
            logs_to_be_flushed: Vec::new(),
            progress: 0.0,
            last_sync: Instant::now(),
            sleep_until,
        };

        self.0.push(worker);

        Ok(())
    }
}

struct StartJobRequest {
    worker_index: usize,
    job_id: String,
    program_byte_code: Vec<u8>,
}

enum Action {
    StartJob(StartJobRequest),
    Ping,
    Pong,
    Disconnect,
}

fn handle_binary(bin_slice: &[u8]) -> Result<Action> {
    // NOTE to others: DO NOT parse messages like this!
    // let segments = &[bin_slice];
    // let message = capnp::message::Reader::new(
    //     capnp::message::SegmentArray::new(segments),
    //     core::default::Default::default(),
    // );

    let message = serialize::read_message(bin_slice, ReaderOptions::new()).unwrap();

    let decoded = message.get_root::<message_to_node::Reader>().unwrap();

    let kind = decoded.get_kind().with_context(|| "failed to determine incoming binary message kind")?;

    let body = decoded.get_body().which().with_context(|| "could not read node ID")?;

    match (kind, body) {
        (MessageToNodeKind::StartJob, body::Which::StartJob(body)) => {
            let body = body?;
            let worker_index = body.get_worker_index() as usize;
            let job_id =
                String::from_utf8(body.get_job_i_d()?.0.to_vec()).with_context(|| "illegal job ID encoding")?;

            let program_byte_code = body.get_program_byte_code()?;

            Ok(Action::StartJob(StartJobRequest {
                worker_index,
                job_id,
                program_byte_code: program_byte_code.to_vec(),
            }))
        }
        (MessageToNodeKind::Pong, body::Which::Empty(_)) => Ok(Action::Pong),
        (MessageToNodeKind::Ping, body::Which::Empty(_)) => Ok(Action::Ping),
        (_, _) => bail!("illegal message received instead of ID"),
    }
}
