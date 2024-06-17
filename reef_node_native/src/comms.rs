// use tungstenite::Message;

// pub(crate) fn log_message(log_kind: u16, worker_index: u16, log_content: &[u8]) -> Message {
//     let mut message = capnp::message::Builder::new_default();
//     let root: reef_protocol_node::message_capnp::message_from_node::Builder = message.init_root();
//     let mut body = root.init_body().init_job_log();
//
//     body.set_worker_index(worker_index);
//     body.set_log_kind(log_kind);
//     body.set_content(log_content);
//
//     let segments = message.get_segments_for_output();
//     let bin = segments.concat();
//
//     Message::Binary(bin)
// }
