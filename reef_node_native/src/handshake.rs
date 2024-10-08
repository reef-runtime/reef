use anyhow::{bail, Context, Result};
use capnp::{message::ReaderOptions, serialize};
use log::debug;
use tungstenite::Message;

use reef_protocol_node::message_capnp::{self, message_to_node, MessageFromNodeKind, MessageToNodeKind};

use crate::WSConn;

#[derive(Debug)]
pub(crate) struct NodeInfo {
    pub(crate) node_id: [u8; 32],
}

// TODO: add timeouts.
fn read_binary(socket: &mut WSConn) -> Result<Vec<u8>> {
    loop {
        let msg = socket.read().with_context(|| "could not read from connection")?;

        match msg {
            Message::Text(_) => {
                bail!("received unexpected message of type `text` instead of handshake initializer")
            }
            Message::Ping(_) => {}
            Message::Pong(_) => {}
            Message::Close(_) => {
                bail!("connection was closed prematurely");
            }
            Message::Frame(_) => unreachable!("got raw frame, this should never happen"),
            Message::Binary(data) => break Ok(data),
        }
    }
}

// TODO: add a timeout here
pub(crate) fn perform(node_name: &str, num_workers: u16, socket: &mut WSConn) -> Result<NodeInfo> {
    //
    // 1. Wait for (and expect) incoming handshake initializer.
    //
    loop {
        let bin = read_binary(socket)?;

        let message = serialize::read_message(bin.as_slice(), ReaderOptions::new()).unwrap();

        let decoded = message.get_root::<message_to_node::Reader>().unwrap();

        let kind = decoded.get_kind().with_context(|| "failed to determine incoming binary message kind")?;

        match kind {
            MessageToNodeKind::InitHandShake => {
                debug!("received handshake initializer...");
                break;
            }
            MessageToNodeKind::Ping => {}
            other => bail!("first binary message from server is not the expected handshake initializer, got {other:?}"),
        }
    }

    //
    // 2. Respond and send node information.
    //

    socket.send(ack_handshake_message(num_workers, node_name)?).with_context(|| "could not send node information")?;

    //
    // 3. Wait until the server has assigned an ID to this node.
    //
    let node_id = {
        loop {
            let bin = read_binary(socket).with_context(|| "could not read node ID")?;

            let reader = serialize::read_message(bin.as_slice(), ReaderOptions::new())
                .with_context(|| "could not read node ID")?;

            let decoded =
                reader.get_root::<message_to_node::Reader>().with_context(|| "could not decode node ID message")?;

            let kind = decoded.get_kind().unwrap();

            match (kind, decoded.get_body().which().with_context(|| "could not read node ID")?) {
                (MessageToNodeKind::AssignId, message_to_node::body::Which::AssignId(id_reader)) => {
                    let id = id_reader.with_context(|| "could not read node IP")?;
                    let id_reader: message_capnp::assign_id_message::Reader = id;
                    let id_vec = id_reader.get_node_id().with_context(|| "failed to read node IP")?.to_vec();

                    let Ok(id_final): Result<[u8; 32], _> = id_vec.try_into() else {
                        bail!("node ID size mismatch or general failure");
                    };

                    break id_final;
                }
                (MessageToNodeKind::Ping, _) => (),
                (_, _) => bail!("illegal message received instead of ID"),
            }
        }
    };

    Ok(NodeInfo { node_id })
}

fn ack_handshake_message(num_workers: u16, node_name: &str) -> Result<Message> {
    let mut message = capnp::message::Builder::new_default();
    let mut encapsulating_message: reef_protocol_node::message_capnp::message_from_node::Builder = message.init_root();
    encapsulating_message.set_kind(MessageFromNodeKind::HandshakeResponse);

    let mut handshake_response = encapsulating_message.get_body().init_handshake_response();

    handshake_response.set_num_workers(num_workers);
    handshake_response.set_node_name(node_name);

    let mut buffer = vec![];
    capnp::serialize::write_message(&mut buffer, &message).with_context(|| "could not encode message")?;
    Ok(Message::Binary(buffer))
}
