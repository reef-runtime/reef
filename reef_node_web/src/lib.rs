use wasm_bindgen::prelude::*;

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

#[derive(Debug, Clone, Default)]
#[wasm_bindgen]
struct StartJobRequest {
    job_id: String,
    dataset_id: String,
    progress: f32,

    program_byte_code: Vec<u8>,
    interpreter_state: Vec<u8>,
}

#[derive(Debug, Clone, Default)]
#[wasm_bindgen]
pub enum NodeMessageKind {
    #[default]
    Ping,

    InitHandShake,
    AssignId,

    StartJob,
    AbortJob,
}

#[derive(Debug, Clone, Default)]
#[wasm_bindgen]
pub struct NodeMessage {
    kind: NodeMessageKind,
    assign_id_data: Option<[u8; 32]>,
    start_job_data: Option<StartJobRequest>,
    abort_job_data: Option<String>,
}

impl NodeMessage {
    fn ping() -> Self {
        Self { kind: NodeMessageKind::Ping, ..Default::default() }
    }

    fn init_hand_shake() -> Self {
        Self { kind: NodeMessageKind::InitHandShake, ..Default::default() }
    }

    fn assign_id(assign_id_data: [u8; 32]) -> Self {
        Self { kind: NodeMessageKind::AssignId, assign_id_data: Some(assign_id_data), ..Default::default() }
    }

    fn start_job(start_job_data: StartJobRequest) -> Self {
        Self { kind: NodeMessageKind::AssignId, start_job_data: Some(start_job_data), ..Default::default() }
    }

    fn abort_job(abort_job_data: String) -> Self {
        Self { kind: NodeMessageKind::AssignId, abort_job_data: Some(abort_job_data), ..Default::default() }
    }
}

#[derive(Debug, Clone, Default)]
#[wasm_bindgen]
pub struct ParseError(String);

impl<E: std::error::Error> From<E> for ParseError {
    fn from(value: E) -> Self {
        Self(value.to_string())
    }
}

impl std::fmt::Display for ParseError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "ParseError({})", self.0)
    }
}

#[wasm_bindgen]
pub fn parse_websocket_data(data: &[u8]) -> Result<NodeMessage, ParseError> {
    use capnp::{message::ReaderOptions, serialize};
    use reef_protocol_node::message_capnp::{
        assign_id_message,
        message_to_node::{self, body},
        MessageToNodeKind,
    };

    let message = serialize::read_message(data, ReaderOptions::new()).unwrap();

    let decoded = message.get_root::<message_to_node::Reader>().unwrap();

    let kind = decoded.get_kind()?;

    let body = decoded.get_body().which()?;

    match (kind, body) {
        (MessageToNodeKind::Ping, body::Which::Empty(_)) => Ok(NodeMessage::ping()),
        (MessageToNodeKind::InitHandShake, _) => Ok(NodeMessage::init_hand_shake()),
        (MessageToNodeKind::AssignId, body::Which::AssignId(id_reader)) => {
            let id = id_reader?;
            let id_reader: assign_id_message::Reader = id;
            let id_vec = id_reader.get_node_id()?.to_vec();

            let Ok(id_final): Result<[u8; 32], _> = id_vec.try_into() else {
                return Err(ParseError("node ID size mismatch or general failure".into()));
            };

            Ok(NodeMessage::assign_id(id_final))
        }
        (MessageToNodeKind::StartJob, body::Which::StartJob(body)) => {
            let body = body?;
            let job_id = String::from_utf8(body.get_job_id()?.0.to_vec())?;

            let dataset_id = String::from_utf8(body.get_dataset_id()?.0.to_vec())?;

            Ok(NodeMessage::start_job(StartJobRequest {
                job_id,
                dataset_id,
                progress: body.get_progress(),

                program_byte_code: body.get_program_byte_code()?.to_vec(),
                interpreter_state: body.get_interpreter_state()?.to_vec(),
            }))
        }
        (MessageToNodeKind::AbortJob, body::Which::AbortJob(body)) => {
            let body = body?;
            let job_id = String::from_utf8(body.get_job_id()?.0.to_vec())?;
            Ok(NodeMessage::abort_job(job_id))
        }
        (_, _) => Err(ParseError("Illegal message received.".into())),
    }
}
