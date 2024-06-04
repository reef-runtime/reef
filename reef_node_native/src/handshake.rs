use std::net::TcpStream;

use anyhow::{bail, Context, Result};
use capnp::{message::ReaderOptions, serialize};
use reef_protocol_node::message_capnp::{self, message_to_node, MessageToNodeKind};
use tungstenite::{stream::MaybeTlsStream, Message, WebSocket};

fn ack_handshake_message(num_workers: u16, node_name: &str) -> Result<Message> {
    let mut message = capnp::message::Builder::new_default();
    let mut root: reef_protocol_node::message_capnp::handshake_respond_message::Builder =
        message.init_root();
    root.set_num_workers(num_workers);
    root.set_node_name(node_name);

    let mut buffer = vec![];
    capnp::serialize::write_message(&mut buffer, &message)
        .with_context(|| "could not encode message")?;
    Ok(Message::Binary(buffer))
}

#[derive(Debug)]
pub(crate) struct NodeInfo {
    pub(crate) node_id: [u8; 32],
}

// TODO: add timeouts.
fn wait_for_binary_ignore_other(
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
) -> Result<Vec<u8>> {
    loop {
        let msg = socket
            .read()
            .with_context(|| "could not read from connection")?;

        match msg {
            Message::Text(_) => {
                bail!("received unexpected mesage of type `text` instead of handshake initializer")
            }
            Message::Ping(_) => {
                todo!("respond to ping")
            }
            Message::Pong(_) => {
                todo!("respond to pong")
            }
            Message::Close(_) => {
                bail!("connection was closed prematurely");
            }
            Message::Frame(_) => unreachable!("got raw frame, this should never happen"),
            Message::Binary(data) => break Ok(data),
        }
    }
}

// TODO: add a timeout here
pub(crate) fn perform(
    node_name: &str,
    num_workers: u16,
    socket: &mut WebSocket<MaybeTlsStream<TcpStream>>,
) -> Result<NodeInfo> {
    //
    // 1. Wait for (and expect) incoming handshake initializer.
    //
    loop {
        let bin = wait_for_binary_ignore_other(socket)?;

        let message = serialize::read_message(bin.as_slice(), ReaderOptions::new()).unwrap();

        let decoded = message
            .get_root::<reef_protocol_node::message_capnp::message_to_node::Reader>()
            .unwrap();

        let kind = decoded
            .get_kind()
            .with_context(|| "failed to determine incoming binary message kind")?;

        match kind {
            MessageToNodeKind::InitHandShake => {
                println!("received handshake initializer...");
                break
            },
            MessageToNodeKind::Ping | MessageToNodeKind::Pong => println!("received ping, waiting for init handshake..."),
            other => bail!("first binary message from server is not the expected handshake initializer, got {other:?}"),
        }
    }

    //
    // 2. Respond and send node information.
    //
    {
        socket
            .send(ack_handshake_message(num_workers, node_name)?)
            .with_context(|| "could not send node information")?;
    }

    //
    // 3. Wait until the server has assigned an ID to this node.
    //
    let node_id = {
        loop {
            let bin =
                wait_for_binary_ignore_other(socket).with_context(|| "could not read node ID")?;

            let reader = serialize::read_message(bin.as_slice(), ReaderOptions::new())
                .with_context(|| "could not read node ID")?;

            let decoded = reader
                .get_root::<reef_protocol_node::message_capnp::message_to_node::Reader>()
                .with_context(|| "could not decode node ID message")?;

            let kind = decoded.get_kind().unwrap();

            match (
                kind,
                decoded
                    .get_body()
                    .which()
                    .with_context(|| "could not read node ID")?,
            ) {
                (
                    MessageToNodeKind::AssignID,
                    message_to_node::body::Which::AssignID(id_reader),
                ) => {
                    let id = id_reader.with_context(|| "could not read node IP")?;
                    let id_reader: message_capnp::assign_i_d_message::Reader = id;
                    let id_vec = id_reader
                        .get_node_i_d()
                        .with_context(|| "failed to read node IP")?
                        .to_vec();

                    let Ok(id_final): Result<[u8; 32], _> = id_vec.try_into() else {
                        bail!("node ID size mismatch or general failure");
                    };

                    break id_final;
                }
                (MessageToNodeKind::Ping | MessageToNodeKind::Pong, _) => (),
                (_, _) => bail!("illegal message received instead of ID"),
            }
        }
    };

    Ok(NodeInfo { node_id })
}
