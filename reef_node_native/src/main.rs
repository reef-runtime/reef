extern crate websocket;

use std::io::stdin;
use std::sync::mpsc::channel;
use std::sync::{Arc, Mutex};
use std::time::Duration;
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

const CODE_SEND_ID: u8 = 0xB1;
const ID_LEN_BYTES: usize = 32;

const CODE_RECV_HANDSHAKE: u8 = 0xA0;

const CODE_PING: u8 = 0xC0;
const CODE_PONG: u8 = 0xC1;

fn u16_into_bytes(v: u16) -> [u8; 2] {
    [((v & 0xFF00) >> 8) as u8, v as u8]
}

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
                OwnedMessage::Text(ref inner) => println!("[SEND] text: {inner:?}"),
                OwnedMessage::Ping(ref bytes) => println!("[SEND] ping: {bytes:?}"),
                OwnedMessage::Pong(ref bytes) => println!("[SEND] pong: {bytes:?}"),
                OwnedMessage::Binary(ref bytes) => {
                    println!(
                        "[SEND] binary: {:?}",
                        bytes
                            .iter()
                            .map(|b| format!("0x{b:x}"))
                            .collect::<Vec<String>>()
                    )
                }
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

    let init_bool_src = Arc::new(Mutex::new(false));

    let init_bool = init_bool_src.clone();
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
                    println!(
                        "[RECV] binary: {:?}",
                        data.iter()
                            .map(|b| format!("0x{b:x}"))
                            .collect::<Vec<String>>()
                    );

                    if data.is_empty() {
                        println!("[RECV] Empty packet");
                        continue;
                    }

                    match data[0] {
                        CODE_INIT_HANDSHAKE => {
                            println!("[RECV] Received handshake init");

                            // TODO: init handshake
                            // If we receive a handshake request, respond to it.

                            let name = "Hello World Node!";
                            let len_name = name.len();

                            if len_name > u16::MAX.into() {
                                panic!("[bug] Name length was larger than u16 max")
                            }

                            let len_name = len_name as u16;

                            let num_workers = num_cpus::get();

                            if num_workers > u16::MAX.into() {
                                panic!("[bug] Num workers was larger than u16 max")
                            }

                            let tx_body = &[
                                &[CODE_RECV_HANDSHAKE],
                                u16_into_bytes(num_workers as u16).as_slice(),
                                u16_into_bytes(len_name).as_slice(),
                                name.as_bytes(),
                            ]
                            .concat();

                            tx_1.send(OwnedMessage::Binary(tx_body.to_owned())).unwrap();
                        }
                        CODE_SEND_ID => {
                            if data.len() != ID_LEN_BYTES + 1 {
                                panic!("[err] Returned ID is not in the expected format");
                            }

                            let id: &[u8] = &data[1..ID_LEN_BYTES];

                            let mut initialized = init_bool.lock().unwrap();
                            *initialized = true;

                            println!("[end handshake]: recv ID: {id:?}");
                        }
                        CODE_PONG => {
                            println!("[RECV] PONG")
                        },
                        other => {
                            println!("[RECV]: Unkown message: {other:?}")
                        }
                    }
                }
                _ => println!("[RECV] other: {:?}", message),
            }
        }
    });

    let tx_ping = tx.clone();
    let init_bool = init_bool_src.clone();

    thread::spawn(move || {
        loop {
            if !*init_bool.lock().unwrap() {
                thread::sleep(Duration::from_secs(1));
                continue;
            }

            tx_ping.send(OwnedMessage::Binary(vec![CODE_PING])).unwrap();

            // TODO: configure this.
            thread::sleep(Duration::from_secs(10));
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
