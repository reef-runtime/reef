use std::sync::{
    atomic::{AtomicU8, Ordering},
    mpsc::{self},
    Arc,
};
use std::thread::{self, JoinHandle};

use reef_interpreter::{
    exec::{CallResultTyped, ExecHandleTyped},
    imports::{Extern, FuncContext, Imports},
    parse_bytes,
    reference::MemoryStringExt,
    Instance,
};
use rkyv::AlignedVec;

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

const ITERATION_CYCLES: usize = 0x10000;

//
// Worker state.
//

#[derive(Debug)]
pub(crate) struct Job {
    pub(crate) id: String,
    pub(crate) signal: Arc<AtomicU8>,
    pub(crate) handle: WorkerThreadHandle,
}

#[derive(Debug, Default)]
pub(crate) struct Worker {
    pub(crate) job: Option<Job>,
}

//
// End worker state.
//

pub(crate) enum FromWorkerMessage {
    Log(String),
    Progress(f32),
}

pub(crate) type WorkerSender = mpsc::Sender<FromWorkerMessage>;

fn setup_interpreter(
    sender: WorkerSender,
    program: &[u8],
    state: Option<&[u8]>,
) -> std::result::Result<ReefMainHandle, reef_interpreter::error::Error> {
    let module = parse_bytes(program)?;

    let mut imports = Imports::new();

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
struct WorkerSignal;

impl WorkerSignal {
    pub const CONTINUE: u8 = 0;
    pub const SAVE_STATE: u8 = 1;
    pub const ABORT: u8 = 2;
}

pub(crate) fn spawn_worker_thread(
    sender: WorkerSender,
    signal: Arc<AtomicU8>,
    program: Vec<u8>,
    job_id: String,
) -> WorkerThreadHandle {
    thread::spawn(move || {
        println!("Instantiating WASM interpreter...");

        // TODO get previous state
        let mut exec_handle = setup_interpreter(sender.clone(), &program, None)?;

        let mut serialized_state: Option<AlignedVec> = None;

        println!("Executing {}...", job_id);

        loop {
            // Check for signal from manager thread
            match signal.swap(0, Ordering::Relaxed) {
                // No signal, perform normal execution.
                WorkerSignal::CONTINUE => (),
                // Perform a state sync.
                WorkerSignal::SAVE_STATE => {
                    if serialized_state.is_none() {
                        serialized_state =
                            Some(AlignedVec::with_capacity(reef_interpreter::PAGE_SIZE * 2));
                    }
                    serialized_state =
                        Some(exec_handle.serialize(serialized_state.take().unwrap())?);

                    println!(
                        "Serialized {} bytes for state of {}.",
                        serialized_state.as_ref().unwrap().len(),
                        job_id
                    );
                }
                // Kill the worker
                WorkerSignal::ABORT => break Err(JobError::Aborted),
                _ => unreachable!("internal bug: master thread has sent invalid signal"),
            }

            // Execute Wasm
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
