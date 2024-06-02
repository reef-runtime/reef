use core::panic;
use std::{
    io::ErrorKind,
    sync::{
        atomic::{AtomicU8, Ordering},
        mpsc::{self},
        Arc,
    },
    thread::{self, JoinHandle},
    time::Duration,
};

use tinywasm::{
    CallResultTyped, ExecHandleTyped, Extern, FuncContext, Imports, Instance, MemoryStringExt,
};

const ENTRY_FN_NAME: &str = "reef_main";

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
    Progress(u16),
}

pub(crate) type WorkerSenderChannel = mpsc::Sender<FromWorkerMessage>;

// type WorkerReceiverChannel = mpsc::Receiver<ToWorkerMessage>;

const IDLE_SLEEP: Duration = Duration::from_secs(1);

// TODO: replace this with a pointer?
// OR: just return a struct directly?
type EntryPointFnReturnValueType = i32;
type EntryPointFnHandle = ExecHandleTyped<EntryPointFnReturnValueType>;

fn setup_interpreter(
    program: &[u8],
    sender: WorkerSenderChannel,
) -> std::result::Result<EntryPointFnHandle, tinywasm::Error> {
    let module = tinywasm::parse_bytes(program)?;

    let mut imports = Imports::new();

    let sender_log = sender.clone();
    imports.define(
        "reef",
        "log",
        Extern::typed_func(move |ctx: FuncContext<'_>, args: (i32, i32)| {
            let mem = ctx.exported_memory("memory")?;
            let ptr = args.0 as usize;
            let len = args.1 as usize;
            let log_string = mem.load_string(ptr, len)?;
            println!("REEF_LOG: {}", log_string);

            sender_log
                .send(FromWorkerMessage::Log(log_string))
                .expect("receiver cannot hang up");

            Ok(())
        }),
    )?;

    imports.define(
        "reef",
        "progress",
        Extern::typed_func(move |mut _ctx: FuncContext<'_>, percent: i32| {
            if !(u16::MIN.into()..=u16::MAX.into()).contains(&percent) {
                return Err(tinywasm::Error::Io(std::io::Error::new(
                    ErrorKind::AddrNotAvailable,
                    "Invalid range: percentage must be in u16 range",
                )));
            }

            println!("REEF_REPORT_PROGRESS: {percent}");

            sender
                .send(FromWorkerMessage::Progress(percent as u16))
                .expect("receiver cannot hang up");

            Ok(())
        }),
    )?;

    let instance = Instance::instantiate(module, imports)?;
    let entry_fn_handle = instance.exported_func::<i32, i32>(ENTRY_FN_NAME).unwrap();
    let exec_handle = entry_fn_handle.call(0)?;

    Ok(exec_handle)
}

#[derive(Debug)]
pub(crate) enum JobError {
    Killed,
    InitializationError(tinywasm::Error),
    RuntimeError(String),
}

impl From<tinywasm::Error> for JobError {
    fn from(err: tinywasm::Error) -> Self {
        Self::InitializationError(err)
    }
}

pub(crate) type WorkerThreadHandle =
    JoinHandle<std::result::Result<EntryPointFnReturnValueType, JobError>>;

#[non_exhaustive]
struct WorkerSignal;

impl WorkerSignal {
    pub const NOP: u8 = 0;
    pub const SAVE_STATE: u8 = 1;
    pub const KILL: u8 = 2;
}

//  NOTE: pausing the interpreter would probably be more efficient if OS signals are used.
pub(crate) fn spawn_worker_thread(
    program: Vec<u8>,
    job_id: String,
    sender: WorkerSenderChannel,
    signal: Arc<AtomicU8>,
) -> WorkerThreadHandle {
    thread::spawn(
        move || -> std::result::Result<EntryPointFnReturnValueType, JobError> {
            println!("Creating WASM sandbox...");
            let mut exec_handle = setup_interpreter(&program, sender.clone())?;
            let max_cycles = 10;
            println!("WASM sandbox created successfully.");

            println!("-> Executing {} ...", job_id);

            loop {
                match signal.swap(0, Ordering::Relaxed) {
                    //
                    // No signal, perform normal execution.
                    //
                    WorkerSignal::NOP => (),
                    //
                    // Perform a state sync.
                    //
                    WorkerSignal::SAVE_STATE => {
                        println!("========================== Would perform a sync now =======================");
                        continue;
                    }
                    //
                    // Kill the worker
                    //
                    WorkerSignal::KILL => break Err(JobError::Killed),
                    _ => unreachable!("internal bug: master thread has sent invalid signal"),
                }

                let run_res = exec_handle.run(max_cycles);
                match run_res {
                    Ok(CallResultTyped::Done(return_value)) => {
                        break Ok(return_value);
                    }
                    Ok(CallResultTyped::Incomplete) => {
                        sender
                            .send(FromWorkerMessage::Log("Executed part of the job".into()))
                            .expect("receiver cannot disconnect");
                        println!("Executed step of {max_cycles} instructions.")
                    }
                    Err(err) => panic!("WASM thread has crashed: {err}"),
                }
            }
        },
    )
}
