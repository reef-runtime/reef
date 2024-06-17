use std::sync::Mutex;
// use std::net::TcpStream;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};
use std::{
    sync::{
        atomic::{AtomicU8, Ordering},
        mpsc, Arc,
    },
    // u32,
};

// use anyhow::Context;
use reef_interpreter::{
    exec::{CallResultTyped, ExecHandleTyped},
    imports::{Extern, FuncContext, Imports},
    parse_bytes,
    reference::MemoryStringExt,
    Instance,
};
use rkyv::AlignedVec;
// use tungstenite::stream::MaybeTlsStream;
// use tungstenite::WebSocket;

use crate::WSConn;

//
// Wasm interface declaration
//

const REEF_MAIN_NAME: &str = "reef_main";
type ReefMainArgs = (i32,);
type ReefMainReturn = (i32,);
type ReefMainHandle = ExecHandleTyped<ReefMainReturn>;

const REEF_LOG_NAME: (&str, &str) = ("reef", "log");
type ReefLogArgs = (i32, i32);
// type ReefLogReturn = ();

const REEF_PROGRESS_NAME: (&str, &str) = ("reef", "progress");
type ReefProgressArgs = (f32,);
// type ReefProgressReturn = ();

const REEF_SLEEP_NAME: (&str, &str) = ("reef", "sleep");
// Seconds to sleep.
type ReefSleepArgs = (f32,);
// type ReefProgressReturn = ();

const ITERATION_CYCLES: usize = 0x10000;

//
// Worker state.
//

#[derive(Debug)]
pub(crate) struct Worker {
    pub(crate) last_sync: Instant,
    pub(crate) sleep_until: Arc<Mutex<Option<Instant>>>,

    pub(crate) worker_index: usize,
    pub(crate) job_id: String,

    pub(crate) signal_to_worker: Arc<AtomicU8>,
    pub(crate) channel_from_worker: mpsc::Receiver<FromWorkerMessage>,

    pub(crate) handle: WorkerThreadHandle,

    pub(crate) logs_to_be_flushed: Vec<String>,
    pub(crate) progress: f32,
}

impl Worker {
    pub(crate) fn flush_state(
        &mut self,
        state: AlignedVec,
        socket: &mut WSConn,
    ) -> anyhow::Result<()> {
        self.last_sync = Instant::now();
        todo!("impl this")
    }
}

//
// End worker state.
//

pub(crate) enum FromWorkerMessage {
    State(AlignedVec),
    Log(String),
    Progress(f32),
    Sleep(f32),
}

pub(crate) type WorkerSender = mpsc::Sender<FromWorkerMessage>;

fn reef_std_lib(
    sender: WorkerSender,
) -> std::result::Result<Imports, reef_interpreter::error::Error> {
    let mut imports = Imports::new();

    //
    // Reef Log.
    //
    let sender_log = sender.clone();
    imports.define(
        REEF_LOG_NAME.0,
        REEF_LOG_NAME.1,
        Extern::typed_func(move |ctx: FuncContext<'_>, args: ReefLogArgs| {
            let mem = ctx.exported_memory("memory")?;
            let ptr = args.0 as usize;
            let len = args.1 as usize;
            let log_string = mem.load_string(ptr, len)?;

            sender_log.send(FromWorkerMessage::Log(log_string)).unwrap();

            Ok(())
        }),
    )?;

    //
    // Reef report progress.
    //
    let sender_progress = sender.clone();
    imports.define(
        REEF_PROGRESS_NAME.0,
        REEF_PROGRESS_NAME.1,
        Extern::typed_func(move |mut _ctx: FuncContext<'_>, done: ReefProgressArgs| {
            if !(0.0..=1.0).contains(&done.0) {
                return Err(reef_interpreter::error::Error::Other(
                    "reef/progress: value not in Range 0.0..=1.0".into(),
                ));
            }

            sender_progress
                .send(FromWorkerMessage::Progress(done.0))
                .unwrap();

            Ok(())
        }),
    )?;

    //
    // Reef sleep.
    //
    let sender_sleep = sender.clone();
    imports.define(
        REEF_SLEEP_NAME.0,
        REEF_SLEEP_NAME.1,
        Extern::typed_func(move |mut _ctx: FuncContext<'_>, done: ReefProgressArgs| {
            sender_sleep.send(FromWorkerMessage::Sleep(done.0)).unwrap();

            Ok(())
        }),
    )?;

    Ok(imports)
}

fn setup_interpreter(
    sender: WorkerSender,
    program: &[u8],
    state: Option<&[u8]>,
    // worker_index: usize,
) -> std::result::Result<ReefMainHandle, reef_interpreter::error::Error> {
    let module = parse_bytes(program)?;
    let imports = reef_std_lib(sender)?;

    let (instance, stack) = Instance::instantiate(module, imports, state)?;
    let entry_fn_handle = instance
        .exported_func::<ReefMainArgs, ReefMainReturn>(REEF_MAIN_NAME)
        .unwrap();
    let exec_handle = entry_fn_handle.call((0,), stack)?;

    Ok(exec_handle)
}

#[derive(Debug)]
pub(crate) enum JobError {
    Aborted,
    Interpreter(reef_interpreter::error::Error),
}

impl From<reef_interpreter::error::Error> for JobError {
    fn from(err: reef_interpreter::error::Error) -> Self {
        Self::Interpreter(err)
    }
}

pub(crate) type WorkerThreadHandle = JoinHandle<Result<ReefMainReturn, JobError>>;

#[non_exhaustive]
pub(crate) struct WorkerSignal;

impl WorkerSignal {
    pub(crate) const SLEEP: u8 = 0;
    pub(crate) const CONTINUE: u8 = 1;
    pub(crate) const SAVE_STATE: u8 = 2;
    pub(crate) const ABORT: u8 = 3;
}

pub(crate) fn spawn_worker_thread(
    sender: WorkerSender,
    signal: Arc<AtomicU8>,
    program: Vec<u8>,
    job_id: String,
    sleep_until: Arc<Mutex<Option<Instant>>>,
    // worker_index: usize,
) -> WorkerThreadHandle {
    thread::spawn(move || -> Result<(i32,), JobError> {
        println!("Instantiating WASM interpreter...");

        // TODO get previous state
        let mut exec_handle = setup_interpreter(sender.clone(), &program, None)?;

        // This is not being re-allocated inside the hotloop for performance gains.
        let mut serialized_state: Option<AlignedVec> = None;

        println!("Executing {}...", job_id);

        let mut sleep = false;

        loop {
            // Check for signal from manager thread.
            match signal.swap(0, Ordering::Relaxed) {
                // Do not perform further execution.
                WorkerSignal::SLEEP => {
                    sleep = true;
                    continue;
                }
                // No signal, perform normal execution.
                WorkerSignal::CONTINUE => (),
                // Perform a state sync.
                WorkerSignal::SAVE_STATE => {
                    serialized_state = match serialized_state.is_some() {
                        true => Some(exec_handle.serialize(serialized_state.take().unwrap())?),
                        false => Some(AlignedVec::with_capacity(reef_interpreter::PAGE_SIZE * 2)),
                    };

                    println!(
                        "Serialized {} bytes for state of {}.",
                        serialized_state.as_ref().unwrap().len(),
                        job_id
                    );

                    sender
                        .send(FromWorkerMessage::State(serialized_state.take().unwrap()))
                        .unwrap();
                }
                // Kill the worker.
                WorkerSignal::ABORT => break Err(JobError::Aborted),
                other => {
                    unreachable!("internal bug: master thread has sent invalid signal: {other}")
                }
            }

            if sleep {
                if Instant::now().duration_since(sleep_until.lock().unwrap().unwrap())
                    != Duration::ZERO
                {
                    continue;
                }

                sleep = false;
            }

            // Execute Wasm.
            let run_res = exec_handle.run(ITERATION_CYCLES);
            match run_res {
                Ok(CallResultTyped::Done(return_value)) => {
                    break Ok(return_value);
                }
                Ok(CallResultTyped::Incomplete) => {}
                Err(err) => return Err(JobError::Interpreter(err)),
            }
        }
    })
}
