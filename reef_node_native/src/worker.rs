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

use reef_interpreter::PAGE_SIZE;
use reef_interpreter::{
    exec::{CallResultTyped, ExecHandleTyped},
    imports::{Extern, FuncContext, Imports},
    parse_bytes,
    reference::MemoryStringExt,
    Instance,
};
use reef_protocol_node::message_capnp::{MessageFromNodeKind, ResultContentType};
use tungstenite::Message;

use crate::WSConn;

// TODO: use a shared constant for this.
const TODO_LOG_KIND_DEFAULT: u16 = 0;

//
// Wasm interface declaration
//

const REEF_MAIN_NAME: &str = "reef_main";
type ReefMainArgs = ();
type ReefMainReturn = ();
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

const REEF_DATASET_LEN_NAME: (&str, &str) = ("reef", "dataset_len");
type ReefDatasetLenArgs = ();
type ReefDatasetLenReturn = (i32,);

const REEF_DATASET_WRITE_NAME: (&str, &str) = ("reef", "dataset_write");
type ReefDatasetWriteArgs = (i32,);
type ReefDatasetWriteReturn = ();

const ITERATION_CYCLES: usize = 0x10000;

const MAX_CONTINUES_SLEEP: Duration = Duration::from_millis(100);

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

pub(crate) struct JobResult {
    pub(crate) success: bool,
    pub(crate) contents: Vec<u8>,
    pub(crate) content_type: ResultContentType,
}

impl Job {
    pub(crate) fn flush_state(&mut self, state: &[u8], socket: &mut WSConn) -> anyhow::Result<()> {
        let mut message = capnp::message::Builder::new_default();
        let mut encapsulating_message: reef_protocol_node::message_capnp::message_from_node::Builder =
            message.init_root();
        encapsulating_message.set_kind(MessageFromNodeKind::JobStateSync);

        let mut state_sync = encapsulating_message.get_body().init_job_state_sync();

        state_sync.set_worker_index(self.worker_index as u16);
        state_sync.set_progress(self.progress);
        state_sync.set_interpreter(state);

        // Logs.
        let mut logs = state_sync.init_logs(self.logs_to_be_flushed.len() as u32);
        let logs_to_flush = mem::take(&mut self.logs_to_be_flushed);

        for (idx, log) in logs_to_flush.into_iter().enumerate() {
            let mut log_item = logs.reborrow().get(idx as u32);
            log_item.set_content(&log.content.into_bytes());
            log_item.set_log_kind(log.kind);
        }

        let mut buffer = vec![];

        capnp::serialize::write_message(&mut buffer, &message).with_context(|| "could not encode message")?;

        socket.write(Message::Binary(buffer)).with_context(|| "could not send state sync")?;

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
    Done,
}

pub(crate) type WorkerSender = mpsc::Sender<FromWorkerMessage>;

fn reef_std_lib(sender: WorkerSender, sleep_until: Rc<Cell<Instant>>) -> Result<Imports, reef_interpreter::Error> {
    let mut imports = Imports::new();

    // Reef Log.
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

    // Reef report progress.
    let sender_progress = sender.clone();
    imports.define(
        REEF_PROGRESS_NAME.0,
        REEF_PROGRESS_NAME.1,
        Extern::typed_func(move |mut _ctx: FuncContext<'_>, (done,): ReefProgressArgs| {
            if !(0.0..=1.0).contains(&done) {
                return Err(reef_interpreter::Error::Other("reef/progress: value not in Range 0.0..=1.0".into()));
            }

            sender_progress.send(FromWorkerMessage::Progress(done)).unwrap();

            Ok(())
        }),
    )?;

    // Reef sleep.
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

    // Reef dataset.
    const TEMP_DATASET_LEN: usize = 100000;
    imports.define(
        REEF_DATASET_LEN_NAME.0,
        REEF_DATASET_LEN_NAME.1,
        Extern::typed_func::<_, ReefDatasetLenReturn>(move |mut _ctx: FuncContext<'_>, _args: ReefDatasetLenArgs| {
            Ok((TEMP_DATASET_LEN as i32,))
        }),
    )?;

    imports.define(
        REEF_DATASET_WRITE_NAME.0,
        REEF_DATASET_WRITE_NAME.1,
        Extern::typed_func::<_, ReefDatasetWriteReturn>(
            move |mut ctx: FuncContext<'_>, (ptr,): ReefDatasetWriteArgs| {
                if ptr as usize % PAGE_SIZE != 0 {
                    println!("WARM: wasm wants dataset written to non page aligned ptr {ptr}");
                }

                let mut mem = ctx.exported_memory_mut("memory")?;
                mem.fill(ptr as usize, TEMP_DATASET_LEN, 69)?;
                let page = (ptr as usize) / PAGE_SIZE;
                let count = TEMP_DATASET_LEN.div_ceil(PAGE_SIZE);
                mem.set_ignored_page_region(page, count);
                Ok(())
            },
        ),
    )?;

    Ok(imports)
}

fn setup_interpreter(
    sender: WorkerSender,
    program: &[u8],
    state: Option<&[u8]>,
    sleep_until: Rc<Cell<Instant>>,
) -> Result<ReefMainHandle, reef_interpreter::Error> {
    let module = parse_bytes(program)?;
    let imports = reef_std_lib(sender, sleep_until)?;

    let (instance, stack) = Instance::instantiate(module, imports, state)?;
    if stack.is_some() {
        // TODO: reload dataset
        // instance.exported_memory_mut("memory")?.copy_into_ignored_page_region(...);
    }

    let entry_fn_handle = instance.exported_func::<ReefMainArgs, ReefMainReturn>(REEF_MAIN_NAME).unwrap();
    let exec_handle = entry_fn_handle.call((), stack)?;

    Ok(exec_handle)
}

type ReefJobOutput = (Vec<u8>, ResultContentType);
pub(crate) type JobThreadHandle = JoinHandle<Result<ReefJobOutput, reef_interpreter::Error>>;

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
    job_id: String,
    program: Vec<u8>,
    state: Option<Vec<u8>>,
) -> JobThreadHandle {
    thread::spawn(move || -> Result<ReefJobOutput, reef_interpreter::Error> {
        println!("Instantiating WASM interpreter...");

        let sleep_until = Rc::new(Cell::new(Instant::now()));

        // TODO get previous state
        let mut exec_handle = match setup_interpreter(sender.clone(), &program, state.as_deref(), sleep_until.clone()) {
            Ok(handle) => handle,
            Err(err) => {
                sender.send(FromWorkerMessage::Done).unwrap();
                return Err(err.into());
            }
        };

        // This is not being re-allocated inside the hotloop for performance gains.
        let mut serialized_state = Vec::with_capacity(PAGE_SIZE * 2);

        println!("Executing {}...", job_id);

        let res = loop {
            // Check for signal from manager thread.
            match signal.swap(WorkerSignal::CONTINUE, Ordering::Relaxed) {
                // No signal, perform normal execution.
                WorkerSignal::CONTINUE => (),
                // Perform a state sync.
                WorkerSignal::SAVE_STATE => {
                    serialized_state.clear();
                    let mut writer = std::io::Cursor::new(&mut serialized_state);
                    exec_handle.serialize(&mut writer)?;

                    println!("Serialized {} bytes for state of {}.", serialized_state.len(), job_id);

                    sender.send(FromWorkerMessage::State(serialized_state.clone())).unwrap();
                }
                // Kill the worker.
                WorkerSignal::ABORT => break Err(reef_interpreter::Error::Other("Job aborted".into())),
                other => {
                    unreachable!("internal bug: master thread has sent invalid signal: {other}")
                }
            }

            let sleep_remaining = sleep_until.get().duration_since(Instant::now());
            if sleep_remaining != Duration::ZERO {
                let dur = sleep_remaining.min(MAX_CONTINUES_SLEEP);
                thread::sleep(dur);
                continue;
            }

            // Execute Wasm.
            let run_res = exec_handle.run(ITERATION_CYCLES);
            match run_res {
                Ok(CallResultTyped::Done(_)) => {
                    // TODO: @konsti, do this.
                    break Ok((vec![1, 2, 3, 4], ResultContentType::Bytes));
                }
                Ok(CallResultTyped::Incomplete) => {}
                Err(err) => break Err(err),
            }
        };

        sender.send(FromWorkerMessage::Done).unwrap();
        res
    })
}
