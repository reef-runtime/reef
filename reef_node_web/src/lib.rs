use wasm_bindgen::prelude::*;

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
    reef_wasm_interface::NODE_REGISTER_PATH.to_owned()
}

struct NodeState(Option<()>);

// needs to take
// program_byte_code: Vec<u8>,
// interpreter_state: Vec<u8>,
// dataset
#[wasm_bindgen]
pub fn init_node() {
    //
}

// max_cycles
// returns time to sleep
#[wasm_bindgen]
pub fn run_node() -> u32 {
    todo!()
}
