use std::cell::Cell;
use std::mem;
use std::rc::Rc;
use std::sync::{
    atomic::{AtomicU8, Ordering},
    mpsc, Arc,
};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use anyhow::Context;

use reef_interpreter::{
    exec::{CallResultTyped, ExecHandleTyped},
    imports::{Extern, FuncContext, Imports},
    parse_bytes,
    reference::MemoryStringExt,
    Instance,
};
use tungstenite::Message;

use crate::WSConn;

// TODO: use a shared constant for this.
const TODO_LOG_KIND_DEFAULT: u16 = 0;

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
type ReefSleepReturn = ();

const ITERATION_CYCLES: usize = 0x10000;

#[derive(Debug)]
pub(crate) struct ReefLog {
    pub(crate) content: String,
    pub(crate) kind: u16,
}

//
// Worker state.
//

#[derive(Debug)]
pub(crate) struct Job {
    pub(crate) last_sync: Instant,

    pub(crate) worker_index: usize,
    pub(crate) job_id: String,

    pub(crate) signal_to_worker: Arc<AtomicU8>,
    pub(crate) channel_from_worker: mpsc::Receiver<FromWorkerMessage>,

    pub(crate) handle: JobThreadHandle,

    pub(crate) logs_to_be_flushed: Vec<ReefLog>,
    pub(crate) progress: f32,
}

impl Job {
    pub(crate) fn flush_state(&mut self, state: &[u8], socket: &mut WSConn) -> anyhow::Result<()> {
        let mut message = capnp::message::Builder::new_default();
        let mut root: reef_protocol_node::message_capnp::job_state_sync::Builder = message.init_root();

        root.set_worker_index(self.worker_index as u16);
        root.set_progress(self.progress);
        root.set_interpreter(state);

        // Logs.
        let mut logs = root.init_logs(self.logs_to_be_flushed.len() as u32);
        let logs_to_flush = mem::take(&mut self.logs_to_be_flushed);

        for (idx, log) in logs_to_flush.into_iter().enumerate() {
            let mut log_item = logs.reborrow().get(idx as u32);
            log_item.set_content(&log.content.into_bytes());
            log_item.set_log_kind(log.kind);
        }

        let mut buffer = vec![];
        capnp::serialize::write_message(&mut buffer, &message).with_context(|| "could not encode message")?;

        socket.send(Message::Binary(buffer)).with_context(|| "could not send state sync")?;

        self.last_sync = Instant::now();

        Ok(())
    }
}

//
// End worker state.
//

pub(crate) enum FromWorkerMessage {
    State(Vec<u8>),
    Log(ReefLog),
    Progress(f32),
}

pub(crate) type WorkerSender = mpsc::Sender<FromWorkerMessage>;

fn reef_std_lib(
    sender: WorkerSender,
    sleep_until: Rc<Cell<Instant>>,
) -> std::result::Result<Imports, reef_interpreter::error::Error> {
    let mut imports = Imports::new();

    //
    // Reef Log.
    //
    let sender_log = sender.clone();
    imports.define(
        REEF_LOG_NAME.0,
        REEF_LOG_NAME.1,
        Extern::typed_func(move |ctx: FuncContext<'_>, (ptr, len): ReefLogArgs| {
            let mem = ctx.exported_memory("memory")?;
            let log_string = mem.load_string(ptr as usize, len as usize)?;

            sender_log
                .send(FromWorkerMessage::Log(ReefLog { content: log_string, kind: TODO_LOG_KIND_DEFAULT }))
                .unwrap();

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
        Extern::typed_func(move |mut _ctx: FuncContext<'_>, (done,): ReefProgressArgs| {
            if !(0.0..=1.0).contains(&done) {
                return Err(reef_interpreter::error::Error::Other(
                    "reef/progress: value not in Range 0.0..=1.0".into(),
                ));
            }

            sender_progress.send(FromWorkerMessage::Progress(done)).unwrap();

            Ok(())
        }),
    )?;

    //
    // Reef sleep.
    //
    imports.define(
        REEF_SLEEP_NAME.0,
        REEF_SLEEP_NAME.1,
        Extern::typed_func::<_, ReefSleepReturn>(move |mut _ctx: FuncContext<'_>, (seconds,): ReefSleepArgs| {
            println!("Sleep");

            sleep_until.set(
                Instant::now()
                    .checked_add(Duration::from_secs_f32(seconds))
                    .ok_or_else(|| reef_interpreter::Error::Other("reef/sleep: Invalid time.".into()))?,
            );

            Err(reef_interpreter::Error::PauseExecution)
        }),
    )?;

    Ok(imports)
}

fn setup_interpreter(
    sender: WorkerSender,
    program: &[u8],
    state: Option<&[u8]>,
    sleep_until: Rc<Cell<Instant>>,
) -> std::result::Result<ReefMainHandle, reef_interpreter::error::Error> {
    let module = parse_bytes(program)?;
    let imports = reef_std_lib(sender, sleep_until)?;

    let (instance, stack) = Instance::instantiate(module, imports, state)?;
    let entry_fn_handle = instance.exported_func::<ReefMainArgs, ReefMainReturn>(REEF_MAIN_NAME).unwrap();
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

pub(crate) type JobThreadHandle = JoinHandle<Result<ReefMainReturn, JobError>>;

#[non_exhaustive]
pub(crate) struct WorkerSignal;

impl WorkerSignal {
    pub(crate) const CONTINUE: u8 = 0;
    pub(crate) const SAVE_STATE: u8 = 1;
    pub(crate) const ABORT: u8 = 2;
}

pub(crate) fn spawn_worker_thread(
    sender: WorkerSender,
    signal: Arc<AtomicU8>,
    program: Vec<u8>,
    job_id: String,
) -> JobThreadHandle {
    thread::spawn(move || -> Result<(i32,), JobError> {
        println!("Instantiating WASM interpreter...");

        let sleep_until = Rc::new(Cell::new(Instant::now()));

        // TODO get previous state
        let mut exec_handle = setup_interpreter(sender.clone(), &program, None, sleep_until.clone())?;

        // This is not being re-allocated inside the hotloop for performance gains.
        let mut serialized_state = Vec::with_capacity(reef_interpreter::PAGE_SIZE * 2);

        println!("Executing {}...", job_id);

        loop {
            // Check for signal from manager thread.
            match signal.swap(WorkerSignal::CONTINUE, Ordering::Relaxed) {
                // No signal, perform normal execution.
                WorkerSignal::CONTINUE => (),
                // Perform a state sync.
                WorkerSignal::SAVE_STATE => {
                    let mut writer = std::io::Cursor::new(&mut serialized_state);
                    exec_handle.serialize(&mut writer)?;

                    println!("Serialized {} bytes for state of {}.", serialized_state.len(), job_id);

                    sender.send(FromWorkerMessage::State(serialized_state.clone())).unwrap();
                }
                // Kill the worker.
                WorkerSignal::ABORT => break Err(JobError::Aborted),
                other => {
                    unreachable!("internal bug: master thread has sent invalid signal: {other}")
                }
            }

            let sleep_remaining = sleep_until.get().duration_since(Instant::now());
            if sleep_remaining != Duration::ZERO {
                let dur = sleep_remaining.min(Duration::from_millis(100));
                thread::sleep(dur);
                continue;
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
