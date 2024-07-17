use std::cell::{Cell, RefCell};
use std::mem;
use std::rc::Rc;
use std::sync::{
    atomic::{AtomicU8, Ordering},
    mpsc, Arc,
};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use anyhow::Context;
use log::debug;
use tungstenite::Message;

use reef_interpreter::{
    exec::CallResultTyped,
    imports::{Extern, Imports},
    parse_bytes,
    reference::MemoryStringExt,
    Instance, PAGE_SIZE,
};
use reef_protocol_node::message_capnp::{MessageFromNodeKind, ResultContentType};
use reef_wasm_interface::*;

use crate::WSConn;

// TODO: use a shared constant for this.
const TODO_LOG_KIND_DEFAULT: u16 = 0;

const ITERATION_CYCLES: usize = 0x10000;

const MAX_CONTINUES_SLEEP: Duration = Duration::from_millis(100);

#[derive(Debug)]
pub(crate) struct ReefLog {
    pub(crate) content: String,
    pub(crate) kind: u16,
}

#[derive(Debug)]
pub(crate) struct Job {
    pub(crate) last_sync: Instant,
    pub(crate) sync_running: bool,

    pub(crate) worker_index: usize,
    pub(crate) job_id: String,

    pub(crate) signal_to_worker: Arc<AtomicU8>,
    pub(crate) channel_from_worker: mpsc::Receiver<FromWorkerMessage>,

    pub(crate) handle: JobThreadHandle,

    pub(crate) logs_to_be_flushed: Vec<ReefLog>,
    pub(crate) progress: f32,
}

#[derive(Debug)]
pub(crate) struct JobResult {
    pub(crate) success: bool,
    pub(crate) content_type: ResultContentType,
    pub(crate) contents: Vec<u8>,
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

        Ok(())
    }
}

pub(crate) enum FromWorkerMessage {
    State(Vec<u8>),
    Log(ReefLog),
    Progress(f32),
    Done,
}

pub(crate) type WorkerSender = mpsc::Sender<FromWorkerMessage>;

#[derive(Debug)]
pub(crate) struct WorkerData {
    pub(crate) sender: WorkerSender,
    pub(crate) program: Vec<u8>,
    pub(crate) state: Option<Vec<u8>>,
    pub(crate) dataset: Vec<u8>,
}

type ReefJobOutput = (ResultContentType, Vec<u8>);
pub(crate) type JobThreadHandle = JoinHandle<Result<ReefJobOutput, reef_interpreter::Error>>;

#[non_exhaustive]
pub(crate) struct WorkerSignal;

impl WorkerSignal {
    pub(crate) const CONTINUE: u8 = 0;
    pub(crate) const SAVE_STATE: u8 = 1;
    pub(crate) const ABORT: u8 = 2;
}

pub(crate) fn spawn_worker_thread(signal: Arc<AtomicU8>, job_id: String, data: WorkerData) -> JobThreadHandle {
    thread::spawn(move || -> Result<ReefJobOutput, reef_interpreter::Error> {
        debug!("Instantiating WASM interpreter...");

        let sleep_until = Rc::new(Cell::new(Instant::now()));
        let job_output = Rc::new(RefCell::new((ResultContentType::Bytes, Vec::new())));

        let sender = data.sender.clone();
        // send initial state sync to move job from starting to running
        sender.send(FromWorkerMessage::State(data.state.clone().unwrap_or_default())).unwrap();

        let mut exec_handle = match setup_interpreter(data, sleep_until.clone(), job_output.clone()) {
            Ok(handle) => handle,
            Err(err) => {
                sender.send(FromWorkerMessage::Done).unwrap();
                return Err(err);
            }
        };

        // This is not being re-allocated inside the hotloop for performance gains.
        let mut serialized_state = Vec::with_capacity(PAGE_SIZE * 2);

        debug!("Executing '{job_id}'...");

        let res = loop {
            // Check for signal from manager thread.
            match signal.swap(WorkerSignal::CONTINUE, Ordering::Relaxed) {
                // No signal, perform normal execution.
                WorkerSignal::CONTINUE => (),
                // Perform a state sync.
                WorkerSignal::SAVE_STATE => {
                    serialized_state.clear();
                    let mut writer = std::io::Cursor::new(&mut serialized_state);

                    // this clone should generally be cheap because job output is either not set or
                    // very short during execution.
                    let mut extra_data = job_output.borrow().1.clone();
                    extra_data.push(reef_wasm_interface::content_type_to_num(job_output.borrow().0));

                    exec_handle.serialize(&mut writer, &job_output.borrow().1)?;

                    debug!("Serialized {} bytes for state of {}.", serialized_state.len(), job_id);

                    sender.send(FromWorkerMessage::State(serialized_state.clone())).unwrap();
                }
                // Kill the worker.
                WorkerSignal::ABORT => break Err(reef_interpreter::Error::Other("job was aborted".into())),
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
                    break Ok(());
                }
                Ok(CallResultTyped::Incomplete) => {}
                Err(err) => break Err(err),
            }
        };

        sender.send(FromWorkerMessage::Done).unwrap();
        drop(exec_handle);

        let job_output = Rc::try_unwrap(job_output).unwrap().into_inner();
        res.map(|_| job_output)
    })
}

fn setup_interpreter(
    data: WorkerData,
    sleep_until: Rc<Cell<Instant>>,
    job_output: Rc<RefCell<(ResultContentType, Vec<u8>)>>,
) -> Result<ReefMainHandle, reef_interpreter::Error> {
    let dataset = Rc::new(data.dataset);

    let module = parse_bytes(&data.program)?;
    let imports = reef_imports(data.sender, sleep_until, job_output.clone(), dataset.clone())?;

    let (mut instance, stack, mut extra_data) = Instance::instantiate(module, imports, data.state.as_deref())?;
    if stack.is_some() {
        // reload dataset
        let mut mem = instance.exported_memory_mut("memory")?;

        if mem.get_ignored_byte_region().1 == dataset.len() {
            mem.copy_into_ignored_byte_region(&dataset);
        }
        drop(dataset);
    }

    job_output.borrow_mut().0 = reef_wasm_interface::num_to_content_type(extra_data.pop().unwrap_or(0))
        .map_err(|_| reef_interpreter::Error::Other("invalid content type in state sync".into()))?;
    job_output.borrow_mut().1 = extra_data;

    let entry_fn_handle = instance.exported_func::<ReefMainArgs, ReefMainReturn>(REEF_MAIN_NAME)?;
    let exec_handle = entry_fn_handle.call((), stack)?;

    Ok(exec_handle)
}

fn reef_imports(
    sender: WorkerSender,
    sleep_until: Rc<Cell<Instant>>,
    job_output: Rc<RefCell<(ResultContentType, Vec<u8>)>>,
    dataset: Rc<Vec<u8>>,
) -> Result<Imports, reef_interpreter::Error> {
    let mut imports = Imports::new();

    // Reef Log.
    let sender_log = sender.clone();
    imports.define(
        REEF_MODULE_NAME,
        REEF_LOG_NAME,
        Extern::typed_func(move |ctx, (ptr, len): ReefLogArgs| {
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
        REEF_MODULE_NAME,
        REEF_PROGRESS_NAME,
        Extern::typed_func(move |_ctx, (done,): ReefProgressArgs| {
            if !(0.0..=1.0).contains(&done) {
                return Err(reef_interpreter::Error::Other("reef/progress: value not in Range 0.0..=1.0".into()));
            }

            sender_progress.send(FromWorkerMessage::Progress(done)).unwrap();

            Ok(())
        }),
    )?;

    // Reef sleep.
    imports.define(
        REEF_MODULE_NAME,
        REEF_SLEEP_NAME,
        Extern::typed_func::<_, ReefSleepReturn>(move |_ctx, (seconds,): ReefSleepArgs| {
            sleep_until.set(
                Instant::now()
                    .checked_add(Duration::from_secs_f32(seconds))
                    .ok_or_else(|| reef_interpreter::Error::Other("reef/sleep: Invalid time.".into()))?,
            );

            Err(reef_interpreter::Error::PauseExecution)
        }),
    )?;

    // Reef dataset.
    // Reef std implementations guarantee, that the dataset is at least 8 byte aligned.
    let dataset_len = dataset.len();
    let dataset = std::cell::RefCell::new(Some(dataset));
    imports.define(
        REEF_MODULE_NAME,
        REEF_DATASET_LEN_NAME,
        Extern::typed_func::<_, ReefDatasetLenReturn>(move |_ctx, _args: ReefDatasetLenArgs| Ok((dataset_len as i32,))),
    )?;
    imports.define(
        REEF_MODULE_NAME,
        REEF_DATASET_WRITE_NAME,
        Extern::typed_func::<_, ReefDatasetWriteReturn>(move |mut ctx, (ptr,): ReefDatasetWriteArgs| {
            let mut mem = ctx.exported_memory_mut("memory")?;

            mem.set_ignored_byte_region(ptr as usize, dataset_len);
            mem.copy_into_ignored_byte_region(dataset.borrow().as_deref().unwrap_or(&Rc::new(Vec::new())));

            // Drop the remaining Rc reference to free the Vec
            if dataset.borrow().is_some() {
                *dataset.borrow_mut() = None;
            }

            Ok(())
        }),
    )?;

    // Reef result.
    imports.define(
        REEF_MODULE_NAME,
        REEF_RESULT_NAME,
        Extern::typed_func::<_, ReefResultReturn>(move |ctx, (result_type, ptr, len): ReefResultArgs| {
            let mem = ctx.exported_memory("memory")?;
            let data = mem.load_vec(ptr as usize, len as usize)?;

            let content_type = match result_type {
                0 => ResultContentType::I32,
                1 => ResultContentType::Bytes,
                2 => ResultContentType::StringPlain,
                3 => ResultContentType::StringJSON,
                _ => return Err(reef_interpreter::Error::Other("invalid ResultContentType".into())),
            };

            *job_output.borrow_mut() = (content_type, data);

            Ok(())
        }),
    )?;

    Ok(imports)
}
