extern crate websocket;

use core::panic;
use std::io::stdin;
use std::sync::mpsc::channel;
use std::{thread, u16};

use clap::Parser;
use websocket::client::ClientBuilder;
use websocket::url::Url;
use websocket::{Message, OwnedMessage};

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

// const CONNECTION: &'static str = "ws://127.0.0.1:2794";

const NODE_REGISTER_PATH: &str = "/api/node/connect";

const CODE_INIT_HANDSHAKE: u8 = 0xB0;
const CODE_RECV_HANDSHAKE: u8 = 0xA0;

fn main() {
    let args = Args::parse();

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

    let client = ClientBuilder::new(connect_url.as_str())
        .unwrap()
        .add_protocol("rust-websocket")
        .connect_insecure()
        .unwrap();

    println!("Successfully connected");

    let (mut receiver, mut sender) = client.split().unwrap();

    let (tx, rx) = channel();
    let tx_1 = tx.clone();

    let send_loop = thread::spawn(move || {
        loop {
            // Send loop.
            let message = match rx.recv() {
                Ok(m) => m,
                Err(e) => {
                    println!("Send Loop: {:?}", e);
                    return;
                }
            };

            match message {
                OwnedMessage::Text(ref inner) => println!("recv text: {inner:?}"),
                OwnedMessage::Ping(ref bytes) => println!("[ping] {bytes:?}"),
                OwnedMessage::Pong(ref bytes) => println!("[pong] {bytes:?}"),
                OwnedMessage::Binary(ref bytes) => println!("recv binary: {bytes:?}"),
                OwnedMessage::Close(_) => {
                    let _ = sender.send_message(&message);
                    return;
                }
            }

            // Send a message.
            match sender.send_message(&message) {
                Ok(()) => (),
                Err(e) => {
                    println!("Send Loop: {:?}", e);
                    let _ = sender.send_message(&Message::close());
                    return;
                }
            }
        }
    });

    let receive_loop = thread::spawn(move || {
        // Receive loop
        for message in receiver.incoming_messages() {
            let message = match message {
                Ok(m) => m,
                Err(e) => {
                    println!("Receive Loop: {:?}", e);
                    let _ = tx_1.send(OwnedMessage::Close(None));
                    return;
                }
            };
            match message {
                OwnedMessage::Close(_) => {
                    // Got a close message, so send a close message and return
                    let _ = tx_1.send(OwnedMessage::Close(None));
                    return;
                }
                OwnedMessage::Ping(data) => {
                    match tx_1.send(OwnedMessage::Pong(data)) {
                        // Send a pong in response
                        Ok(()) => (),
                        Err(e) => {
                            println!("Receive Loop: {:?}", e);
                            return;
                        }
                    }
                }
                OwnedMessage::Binary(ref data) => {
                    if data.is_empty() {
                        println!("[recv] Empty packet");
                        continue;
                    }

                    match data[0] {
                        CODE_INIT_HANDSHAKE => {
                            // TODO: init handshake
                            // If we receive a handshake request, respond to it.

                            let name = "Hello World Node!";
                            let len_name = name.len();

                            if len_name > u16::MAX.into() {
                                panic!("[bug] Name was larger than u16 max")
                            }

                            let len_name = len_name as u16;

                            let node_info = vec![
                                CODE_RECV_HANDSHAKE,
                                ((len_name & 0xFF00) >> 8) as u8,
                                len_name as u8,
                            ];

                            let node_info = node_info.as_slice();
                            let tx_body = &[node_info, name.as_bytes()].concat();

                            tx_1.send(OwnedMessage::Binary(tx_body.to_owned())).unwrap();
                        }
                        other => {
                            println!("[recv]: Unkown message: {other:?}")
                        }
                    }
                }
                // Say what we received
                _ => println!("Receive Loop: {:?}", message),
            }
        }
    });

    loop {
        let mut input = String::new();

        stdin().read_line(&mut input).unwrap();

        let trimmed = input.trim();

        let message = match trimmed {
            "/close" => {
                // Close the connection
                let _ = tx.send(OwnedMessage::Close(None));
                break;
            }
            // Send a ping
            "/ping" => OwnedMessage::Ping(b"PING".to_vec()),
            // Otherwise, just send text
            _ => OwnedMessage::Text(trimmed.to_string()),
        };

        match tx.send(message) {
            Ok(()) => (),
            Err(e) => {
                println!("Main Loop: {:?}", e);
                break;
            }
        }
    }

    println!("Waiting for child threads to exit");

    let _ = send_loop.join();
    let _ = receive_loop.join();

    println!("Exited");
}
