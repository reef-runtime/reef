use std::{net::TcpStream, u16};

use anyhow::{bail, Context, Ok, Result};
use tungstenite::{stream::MaybeTlsStream, Message, WebSocket};

use clap::Parser;
use url::Url;

mod handshake;

const NODE_REGISTER_PATH: &str = "/api/node/connect";

/// Reef worker node (native)
#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    #[arg(short, long)]
    // Name to be sent to the manager (default is the hostname + extra infos)
    node_name: Option<String>,

    #[arg(short = 'u', long)]
    // Base url of the manager.
    manager_url: Url,

    #[arg(short = 't', long)]
    // Whether to use https to connect to the manager.
    tls: bool,
}

fn log_message(log_kind: u16, worker_index: u16, log_content: &[u8]) -> Message {
    let mut message = capnp::message::Builder::new_default();
    let root: reef_protocol::message_capnp::message_from_node::Builder = message.init_root();
    let mut body = root.init_body().init_job_log();

    // let mut root_log: reef_protocol::message_capnp::job_log_message::Builder = root_generic.init_body();

    body.set_worker_index(worker_index);
    body.set_log_kind(log_kind);
    body.set_content(log_content);

    // let content = root.init_content(log_content.by);
    // content.copy_from_slice(log_content);
    // content.se(content);

    // let mut numbers = root.init_numbers(10);
    // let len = numbers.len();
    // for ii in 0..len {
    // numbers.set(ii, ii as i16);
    // expected_total += ii as i32;
    // }

    let segments = message.get_segments_for_output();
    let bin = segments.concat();

    Message::Binary(bin)
}

/// A WebSocket echo server
fn main() -> anyhow::Result<()> {
    let args = Args::parse();

    //
    // Create connection.
    //

    let scheme = match args.tls {
        true => "wss",
        false => "ws",
    };

    let mut connect_url = args.manager_url.clone();
    connect_url.set_path(NODE_REGISTER_PATH);
    connect_url
        .set_scheme(scheme)
        .expect("ws/wss is always a valid scheme");

    println!("Connecting to {}...", &connect_url);

    env_logger::init();

    let (mut socket, response) = tungstenite::connect(connect_url).expect("Can't connect");

    println!("Connected to the server");
    println!("Response HTTP code: {}", response.status());
    println!("Response contains the following headers:");
    for (ref header, _value) in response.headers() {
        println!("* {}", header);
    }

    //
    // Block in order to perform the handshake.
    //

    let num_workers = 42;
    let node_name = "Hello";

    handshake::perform(node_name, num_workers, &mut socket).with_context(|| "handshake failed")?;

    socket.send(log_message(2, 42, "Hallo".as_bytes())).unwrap();

    loop {
        let msg = socket.read().expect("Error reading message");
        println!("Received: {}", msg);
    }
    // socket.close(None);
}
