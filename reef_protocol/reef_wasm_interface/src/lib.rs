//
// Wasm interface declaration
//

// Entrypoint
pub const REEF_MAIN_NAME: &str = "reef_main";
pub type ReefMainArgs = ();
pub type ReefMainReturn = ();
pub type ReefMainHandle = reef_interpreter::exec::ExecHandleTyped<ReefMainReturn>;

// Imports
pub const REEF_MODULE_NAME: &str = "reef";

pub const REEF_LOG_NAME: &str = "log";
pub type ReefLogArgs = (i32, i32);
// type ReefLogReturn = ();

pub const REEF_PROGRESS_NAME: &str = "progress";
pub type ReefProgressArgs = (f32,);
// type ReefProgressReturn = ();

pub const REEF_SLEEP_NAME: &str = "sleep";
// Seconds to sleep.
pub type ReefSleepArgs = (f32,);
pub type ReefSleepReturn = ();

pub const REEF_DATASET_LEN_NAME: &str = "dataset_len";
pub type ReefDatasetLenArgs = ();
pub type ReefDatasetLenReturn = (i32,);

pub const REEF_DATASET_WRITE_NAME: &str = "dataset_write";
pub type ReefDatasetWriteArgs = (i32,);
pub type ReefDatasetWriteReturn = ();

pub const REEF_RESULT_NAME: &str = "result";
pub type ReefResultArgs = (i32, i32, i32);
pub type ReefResultReturn = ();

//
// API definitions
//
pub const NODE_REGISTER_PATH: &str = "/api/node/connect";
