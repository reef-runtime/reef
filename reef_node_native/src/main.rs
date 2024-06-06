use crate::worker::{spawn_worker_thread, Worker};
use std::{
    sync::{
        atomic::AtomicU8,
        mpsc::{self, TryRecvError},
        Arc,
    },
    thread,
    time::Duration,
    u16,
};

use anyhow::{bail, Context, Result};
use capnp::{message::ReaderOptions, serialize};
use reef_protocol_node::message_capnp::{
    message_to_node::{self, body},
    MessageToNodeKind,
};
use tungstenite::Message;

use clap::Parser;
use url::Url;
use worker::FromWorkerMessage;

use crate::worker::Job;

mod comms;
mod handshake;
mod worker;

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

    #[arg(short = 't', long)]
    // Whether to use https to connect to the manager.
    tls: bool,
}

struct NodeState {
    workers: Box<[Worker]>,
    from_worker_sender: mpsc::Sender<FromWorkerMessage>,
}

impl NodeState {
    fn new(num_workers: u16, sender: mpsc::Sender<FromWorkerMessage>) -> Self {
        let mut workers = Vec::with_capacity(num_workers as usize);

        for _ in 0..num_workers {
            workers.push(Worker::default());
        }

        Self {
            workers: workers.into(),
            from_worker_sender: sender,
        }
    }
}

/// A WebSocket echo server
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
    connect_url
        .set_scheme(scheme)
        .expect("ws/wss is always a valid scheme");

    println!("Connecting to {}...", &connect_url);

    env_logger::init();

    let (mut socket, response) = tungstenite::connect(connect_url).expect("Can't connect");

    println!("  -> Connected to the manager");
    println!(
        "  -> Registration response HTTP code: {}",
        response.status()
    );
    println!("  -> Response contains the following headers:");
    for (ref header, _value) in response.headers() {
        println!("* {}", header);
    }

    //
    // Block in order to perform the handshake.
    //

    let num_workers = std::thread::available_parallelism()
        .with_context(|| "failed to determine number of workers")?
        .get() as u16;

    let node_name = match args.node_name {
        Some(from_args) => from_args,
        None => {
            let hostname = sysinfo::System::host_name()
                .with_context(|| "failed to determine system hostname")?;
            format!("native@{hostname}")
        }
    };

    let (from_node_sender, from_node_receiver) = mpsc::channel();

    let mut state = NodeState::new(num_workers, from_node_sender);

    let node_info = handshake::perform(&node_name, num_workers, &mut socket)
        .with_context(|| "handshake failed")?;

    println!(
        "===> Handshake successful: node {} is connected.",
        hex::encode(node_info.node_id)
    );

    //
    // Buffer for state + logs + progress
    // TODO: this feature is quite important.
    //

    loop {
        //
        // Websocket communications.
        //
        if socket.can_read() {
            let msg = socket.read().expect("Error reading message");
            state
                .handle_websocket(msg)
                .with_context(|| "evaluating incoming message")?;
        }

        //
        // Worker channel communications.
        //

        match from_node_receiver.try_recv() {
            Ok(FromWorkerMessage::Log(contents)) => todo!("implement logging: {contents}"),
            Ok(FromWorkerMessage::Progress(progress)) => todo!("implement progress: {progress}"),
            Err(TryRecvError::Empty) => thread::sleep(MAIN_THREAD_SLEEP),
            Err(TryRecvError::Disconnected) => unreachable!("all senders have been dropped"),
        }

        //
        // Checking whether a thread has finished.
        //

        for worker in state.workers.iter_mut() {
            let Some(job) = worker.job.as_ref() else {
                continue;
            };

            if !job.handle.is_finished() {
                continue;
            }

            let res = worker
                .job
                .take()
                .expect("existence of the job was checked above")
                .handle
                .join()
                .expect("worker thread panic'ed, this is a bug");

            match res {
                Ok(ret_val) => println!("Job has executed successfully! {ret_val}"),
                Err(err) => println!("Job failed: {err:?}"),
            }
        }
    }

    // TODO: implement a close (also on reef protocol layer)
    // socket.close(None);
}

impl NodeState {
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
        let Some(worker) = self.workers.get_mut(request.worker_index as usize) else {
            bail!("requested illegal worker index");
        };

        if worker.job.is_some() {
            bail!("worker is occupied");
        }

        println!(
            "starting job with id {:?} on worker {} with program: {:?}...",
            request.job_id, request.worker_index, request.program_byte_code
        );

        let signal = Arc::new(AtomicU8::new(0));

        let handle = spawn_worker_thread(
            request.program_byte_code,
            request.job_id.clone(),
            self.from_worker_sender.clone(),
            signal.clone(),
        );

        worker.job = Some(Job {
            id: request.job_id,
            signal,
            handle,
        });

        Ok(())
    }
}

struct StartJobRequest {
    worker_index: u32,
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

    let kind = decoded
        .get_kind()
        .with_context(|| "failed to determine incoming binary message kind")?;

    let body = decoded
        .get_body()
        .which()
        .with_context(|| "could not read node ID")?;

    match (kind, body) {
        (MessageToNodeKind::StartJob, body::Which::StartJob(body)) => {
            let body = body?;
            let worker_index = body.get_worker_index();
            let job_id = String::from_utf8(body.get_job_i_d()?.0.to_vec())
                .with_context(|| "illegal job ID encoding")?;

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
