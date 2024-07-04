use std::fmt::Display;
use std::sync::atomic::Ordering;
use std::thread;
use std::time::{Duration, Instant};
use std::{
    net::TcpStream,
    sync::{atomic::AtomicU8, mpsc, Arc},
};

use anyhow::{bail, Context, Result};
use capnp::{message::ReaderOptions, serialize};
use clap::Parser;
use reef_protocol_node::message_capnp::{MessageFromNodeKind, ResultContentType};
use tungstenite::{stream::MaybeTlsStream, Message, WebSocket};
use url::Url;

use reef_protocol_node::message_capnp::{
    message_to_node::{self, body},
    MessageToNodeKind,
};

mod handshake;
mod worker;
use worker::{FromWorkerMessage, Job, WorkerData};

type WSConn = WebSocket<MaybeTlsStream<TcpStream>>;

use crate::worker::{spawn_worker_thread, JobResult, WorkerSignal};

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

    #[arg(short = 'w', long)]
    // How many concurrent workers to offer, default is the number of CPUs.
    num_workers: Option<usize>,
}

struct NodeState(Vec<Job>);

impl Display for NodeState {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "[{}]", self.0.iter().map(|w| { w.job_id.clone() }).collect::<Vec<String>>().join(", "))
    }
}

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

    let (mut socket, response) = tungstenite::connect(connect_url).with_context(|| "Websocket connection")?;

    println!("Connected to the manager");
    println!("Registration response HTTP code: {}", response.status());

    //
    // Perform handshake.
    //
    let num_workers =
        args.num_workers.unwrap_or_else(|| std::thread::available_parallelism().map(|n| n.get()).unwrap_or(1));

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

    println!("==> Handshake successful: node '{}' is connected.", hex::encode(node_info.node_id));

    // switch to non blocking after handshake
    match socket.get_mut() {
        MaybeTlsStream::Plain(stream) => {
            stream.set_nonblocking(true).unwrap();
        }
        _ => {
            panic!("Unknown stream type!");
        }
    }

    let sync_wait_duration = Duration::from_millis(args.sync_delay_millis as u64);

    let mut worked;
    loop {
        worked = false;

        // Websocket communications.
        match socket.read() {
            Ok(msg) => {
                state
                    .handle_websocket(msg, args.manager_url.as_str())
                    .with_context(|| "evaluating incoming message")?;
                worked = true;
            }
            Err(tungstenite::Error::Io(ref err)) if err.kind() == std::io::ErrorKind::WouldBlock => {}
            Err(err) => {
                return Err(err).with_context(|| "reading socket");
            }
        }

        //
        // Worker channels communications.
        //
        let mut finished_worker_indices = vec![];
        for job in state.0.iter_mut() {
            // read channel until empty
            loop {
                let msg = job.channel_from_worker.try_recv();
                if msg.is_ok() {
                    worked = true
                }
                match msg {
                    Ok(FromWorkerMessage::Log(log)) => {
                        println!("recv log: [{}:{}] '{}'", job.worker_index, log.kind, log.content,);

                        job.logs_to_be_flushed.push(log);
                    }
                    Ok(FromWorkerMessage::Progress(new)) => {
                        job.progress = new;
                    }
                    Ok(FromWorkerMessage::State(interpreter_state)) => {
                        job.flush_state(&interpreter_state, &mut socket)?;
                        job.last_sync = Instant::now();
                        job.sync_running = false;
                    }
                    Ok(FromWorkerMessage::Done) => {
                        finished_worker_indices.push(job.worker_index);
                    }
                    // either empty or disconnected
                    Err(_) => break,
                }
            }

            // Send a sync request if enough time has passed.
            let since_last_sync = Instant::now().duration_since(job.last_sync);
            if since_last_sync >= sync_wait_duration && !job.sync_running {
                job.sync_running = true;
                job.signal_to_worker.store(WorkerSignal::SAVE_STATE, Ordering::Relaxed);
                worked = true;
            }
        }

        // Remove all finished jobs.
        for worker_idx in finished_worker_indices {
            let idx_in_vec = state.0.iter().position(|w| w.worker_index == worker_idx).unwrap();

            let mut worker = state.0.remove(idx_in_vec);

            // Transfer any logs and the final progress reading to the manager.
            // State can be empty since it is not required anymore.
            worker.flush_state(&[], &mut socket)?;

            let worker_index = worker.worker_index as u16;

            let thread_res = worker.handle.join().expect("worker thread panic'ed, this is a bug");

            let job_result = match thread_res {
                Ok((content_type, contents)) => {
                    println!("==> Job has executed successfully!");
                    JobResult { success: true, content_type, contents }
                }
                Err(err) => {
                    println!("==> Job failed: {err:?}");
                    JobResult {
                        success: false,
                        content_type: ResultContentType::StringPlain,
                        contents: format!("{:?}", err).into_bytes(),
                    }
                }
            };

            send_job_result(worker_index, &job_result, &mut socket)
                .with_context(|| "could not send final job result to manager")?;
        }

        socket.flush()?;

        if !worked {
            thread::sleep(MAIN_THREAD_SLEEP);
        }
    }

    // TODO: implement a close (also on reef protocol layer)
    // socket.close(None);
}

fn send_job_result(worker_index: u16, res: &JobResult, socket: &mut WSConn) -> anyhow::Result<()> {
    let mut message = capnp::message::Builder::new_default();
    let mut encapsulating_message: reef_protocol_node::message_capnp::message_from_node::Builder = message.init_root();
    encapsulating_message.set_kind(MessageFromNodeKind::JobResult);
    let mut state_result = encapsulating_message.get_body().init_job_result();

    state_result.set_worker_index(worker_index);
    state_result.set_success(res.success);
    state_result.set_contents(&res.contents);
    state_result.set_content_type(res.content_type);

    let mut buffer = vec![];

    capnp::serialize::write_message(&mut buffer, &message).with_context(|| "could not encode message")?;

    socket.write(Message::Binary(buffer)).with_context(|| "could not job result")?;

    Ok(())
}

impl NodeState {
    fn worker_exists(&self, worker_index: usize) -> bool {
        self.0.iter().any(|w| w.worker_index == worker_index)
    }

    fn handle_websocket(&mut self, msg: tungstenite::Message, manager_url: &str) -> Result<()> {
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
                if let Err(err) = self.start_job(request, manager_url) {
                    // TODO: replace with `real` logger.
                    eprintln!("Failed to start job: {err}");
                }
            }
            Action::AbortJob(job_id) => {
                self.abort_job(&job_id)?;
            }
            Action::Ping => {
                println!("received ping, would send pong here...");
            }
            Action::Pong => {
                print!("received pong, doing nothing...");
            }
            Action::Disconnect => bail!("disconnected: connection lost"),
        }

        Ok(())
    }

    fn abort_job(&mut self, job_id: &str) -> Result<()> {
        let Some(job) = self.0.iter().find(|j| j.job_id == job_id) else {
            bail!("job to be aborted with ID {job_id} not found on this node")
        };

        job.signal_to_worker.store(WorkerSignal::ABORT, Ordering::Relaxed);

        Ok(())
    }

    fn start_job(&mut self, request: StartJobRequest, manager_url: &str) -> Result<()> {
        // 1. Check if the worker exists and is available.
        if self.worker_exists(request.worker_index) {
            bail!("requested illegal worker index");
        }

        println!(
            "==> Starting job with id '{:?}' on worker {} with program: [{}]{:?}...",
            request.job_id,
            request.worker_index,
            request.program_byte_code.len(),
            &request.program_byte_code[0..20]
        );

        let signal = Arc::new(AtomicU8::new(0));

        let (to_master_sender, from_worker_receiver) = mpsc::channel();

        let state = if request.interpreter_state.is_empty() { None } else { Some(request.interpreter_state) };

        println!("Fetching dataset '{}'...", request.dataset_id);

        let url = format!("{}api/dataset/{}", manager_url, request.dataset_id);
        let resp = reqwest::blocking::get(url)?;
        let dataset = resp.bytes()?.to_vec();

        let handle = spawn_worker_thread(
            signal.clone(),
            request.job_id.clone(),
            WorkerData { sender: to_master_sender, program: request.program_byte_code, state, dataset },
        );

        let job = Job {
            last_sync: Instant::now(),
            sync_running: false,

            worker_index: request.worker_index,
            job_id: request.job_id,

            signal_to_worker: signal.clone(),
            channel_from_worker: from_worker_receiver,

            handle,

            logs_to_be_flushed: Vec::new(),
            progress: request.progress,
        };

        self.0.push(job);

        Ok(())
    }
}

struct StartJobRequest {
    worker_index: usize,
    job_id: String,
    dataset_id: String,
    progress: f32,

    program_byte_code: Vec<u8>,
    interpreter_state: Vec<u8>,
}

enum Action {
    StartJob(StartJobRequest),
    AbortJob(String),
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
            let job_id = String::from_utf8(body.get_job_id()?.0.to_vec()).with_context(|| "illegal job ID encoding")?;

            let dataset_id =
                String::from_utf8(body.get_dataset_id()?.0.to_vec()).with_context(|| "illegal dataset ID encoding")?;

            Ok(Action::StartJob(StartJobRequest {
                worker_index: body.get_worker_index() as usize,
                job_id,
                dataset_id,
                progress: body.get_progress(),

                program_byte_code: body.get_program_byte_code()?.to_vec(),
                interpreter_state: body.get_interpreter_state()?.to_vec(),
            }))
        }
        (MessageToNodeKind::AbortJob, body::Which::AbortJob(body)) => {
            let body = body?;
            let job_id = String::from_utf8(body.get_job_id()?.0.to_vec()).with_context(|| "illegal job ID encoding")?;
            Ok(Action::AbortJob(job_id))
        }
        (MessageToNodeKind::Pong, body::Which::Empty(_)) => Ok(Action::Pong),
        (MessageToNodeKind::Ping, body::Which::Empty(_)) => Ok(Action::Ping),
        (_, _) => bail!("Illegal message received instead of Job control."),
    }
}
