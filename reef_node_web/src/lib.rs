#![feature(sync_unsafe_cell)]

use std::{
    cell::{Cell, RefCell, SyncUnsafeCell},
    rc::Rc,
};

use wasm_bindgen::prelude::*;

use reef_interpreter::{
    exec::CallResultTyped,
    imports::{Extern, Imports},
    parse_bytes,
    reference::MemoryStringExt,
    Instance,
};
use reef_wasm_interface::*;

pub mod message;
pub use message::*;

#[wasm_bindgen(start)]
pub fn start() {
    std::panic::set_hook(Box::new(console_error_panic_hook::hook));
}

#[wasm_bindgen]
extern "C" {
    #[wasm_bindgen(js_namespace = console)]
    fn log(s: &str);
}

#[wasm_bindgen]
pub fn get_connect_path() -> String {
    NODE_REGISTER_PATH.to_owned()
}

#[derive(Debug)]
struct NodeState {
    handle: ReefMainHandle,
    sleep_for: Rc<Cell<f32>>,
    job_output: Rc<RefCell<JobOutput>>,
}

#[derive(Debug, Clone)]
#[wasm_bindgen(getter_with_clone)]
pub struct JobOutput {
    pub content_type: u16,
    pub data: Vec<u8>,
}

// SAFETY: this code is only ever expected to run in a single threaded environment
unsafe impl Sync for NodeState {}

static NODE_STATE: SyncUnsafeCell<Option<Box<NodeState>>> = SyncUnsafeCell::new(None);

#[wasm_bindgen]
pub fn reset_node() {
    // SAFETY: no other call can be running at the same time
    unsafe { *NODE_STATE.get() = None };
}

#[wasm_bindgen]
pub fn init_node(
    program: &[u8],
    state: &[u8],
    dataset: Vec<u8>,
    log_callback: js_sys::Function,
    progress_callback: js_sys::Function,
) -> Result<(), String> {
    init_node_inner(program, state, dataset, log_callback, progress_callback).map_err(|e| e.to_string())
}

fn init_node_inner(
    program: &[u8],
    state: &[u8],
    dataset: Vec<u8>,
    log_callback: js_sys::Function,
    progress_callback: js_sys::Function,
) -> Result<(), reef_interpreter::Error> {
    let module = parse_bytes(&program)?;

    let sleep_for = Rc::new(Cell::new(0.0));
    let dataset = Rc::new(dataset);
    let job_output = Rc::new(RefCell::new(JobOutput { content_type: 0, data: Vec::new() }));

    let imports =
        reef_imports(log_callback, progress_callback, sleep_for.clone(), dataset.clone(), job_output.clone())?;

    let state = if state.is_empty() { None } else { Some(state) };
    let (mut instance, stack, extra_data) = Instance::instantiate(module, imports, state)?;
    if stack.is_some() {
        // reload dataset
        let mut mem = instance.exported_memory_mut("memory")?;

        if mem.get_ignored_byte_region().1 == dataset.len() {
            mem.copy_into_ignored_byte_region(&dataset);
        }
        drop(dataset);
    }

    job_output.borrow_mut().data = extra_data;

    let entry_fn_handle = instance.exported_func::<ReefMainArgs, ReefMainReturn>(REEF_MAIN_NAME)?;
    let exec_handle = entry_fn_handle.call((), stack)?;

    let node_state = NodeState { handle: exec_handle, sleep_for, job_output };

    // SAFETY: no other call can be running at the same time
    unsafe { *NODE_STATE.get() = Some(Box::new(node_state)) }

    Ok(())
}

fn reef_imports(
    log_callback: js_sys::Function,
    progress_callback: js_sys::Function,
    sleep_for: Rc<Cell<f32>>,
    dataset: Rc<Vec<u8>>,
    job_output: Rc<RefCell<JobOutput>>,
) -> Result<Imports, reef_interpreter::Error> {
    let mut imports = Imports::new();

    // Reef Log.
    imports.define(
        REEF_MODULE_NAME,
        REEF_LOG_NAME,
        Extern::typed_func(move |ctx, (ptr, len): ReefLogArgs| {
            let mem = ctx.exported_memory("memory")?;
            let log_string = mem.load_string(ptr as usize, len as usize)?;

            let this = JsValue::null();
            let log_string = JsValue::from(log_string);
            let _ = log_callback.call1(&this, &log_string);

            Ok(())
        }),
    )?;

    // Reef report progress.
    imports.define(
        REEF_MODULE_NAME,
        REEF_PROGRESS_NAME,
        Extern::typed_func(move |_ctx, (done,): ReefProgressArgs| {
            if !(0.0..=1.0).contains(&done) {
                return Err(reef_interpreter::Error::Other("reef/progress: value not in Range 0.0..=1.0".into()));
            }

            let this = JsValue::null();
            let done = JsValue::from(done);
            let _ = progress_callback.call1(&this, &done);

            Ok(())
        }),
    )?;

    // Reef sleep.
    imports.define(
        REEF_MODULE_NAME,
        REEF_SLEEP_NAME,
        Extern::typed_func::<_, ReefSleepReturn>(move |_ctx, (seconds,): ReefSleepArgs| {
            sleep_for.set(seconds);
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
                0..3 => result_type as u16,
                _ => return Err(reef_interpreter::Error::Other("invalid ResultContentType".into())),
            };

            *job_output.borrow_mut() = JobOutput { content_type, data };

            Ok(())
        }),
    )?;

    Ok(imports)
}

#[derive(Debug, Clone, Default)]
#[wasm_bindgen(getter_with_clone)]
pub struct RunResult {
    pub done: bool,
    pub sleep_for: Option<f32>,
    pub job_output: Option<JobOutput>,
}

#[wasm_bindgen]
pub fn run_node(max_cycles: usize) -> Result<RunResult, String> {
    // SAFETY: no other call can be running at the same time
    let mut node_state = unsafe { (*NODE_STATE.get()).take().unwrap() };

    let run_res = node_state.handle.run(max_cycles);
    match run_res {
        Ok(CallResultTyped::Done(_)) => {
            drop(node_state.handle);
            let job_output = Rc::try_unwrap(node_state.job_output).unwrap().into_inner();
            Ok(RunResult { done: true, sleep_for: None, job_output: Some(job_output) })
        }
        Ok(CallResultTyped::Incomplete) => {
            let sleep_for = node_state.sleep_for.replace(0.0);
            unsafe { *NODE_STATE.get() = Some(node_state) }

            Ok(RunResult { done: false, sleep_for: Some(sleep_for), job_output: None })
        }
        Err(err) => Err(err.to_string()),
    }
}

#[wasm_bindgen]
pub fn serialize_state() -> Vec<u8> {
    let mut buffer = Vec::new();
    let mut writer = std::io::Cursor::new(&mut buffer);

    // SAFETY: no other call can be running at the same time
    let mut node_state = unsafe { (*NODE_STATE.get()).take().unwrap() };

    node_state.handle.serialize_raw(&mut writer, &node_state.job_output.borrow().data).unwrap();

    unsafe { *NODE_STATE.get() = Some(node_state) }

    buffer
}
