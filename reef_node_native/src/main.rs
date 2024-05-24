extern crate websocket;

use core::panic;
use std::io::stdin;
use std::sync::mpsc::channel;
use std::sync::{Arc, Mutex};
use std::time::Duration;
use std::{thread, u16, u32};

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

const CODE_PING: u8 = 0xF0;
const CODE_PONG: u8 = 0xF1;

const CODE_JOB_LOG: u8 = 0xD0;

const CODE_START_JOB: u8 = 0xC0;
const CODE_STARTED_JOB: u8 = 0xC1;
const CODE_REJECTED_JOB: u8 = 0xC2;

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
                    panic!("receive err: {e}");
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
                    panic!("received close");
                    return;
                }
            }

            // Send a message.
            match sender.send_message(&message) {
                Ok(()) => (),
                Err(e) => {
                    println!("Send Loop: {:?}", e);
                    let _ = sender.send_message(&Message::close());
                    println!("send err: {e}");
                    return;
                }
            }
        }
    });

    let worker_raw: Option<u16> = None;
    let mut worker_arc = Arc::new(Mutex::new(worker_raw));

    let init_bool_src = Arc::new(Mutex::new(false));

    let init_bool = init_bool_src.clone();

    let mut worker_recv = worker_arc.clone();

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
                        CODE_START_JOB => {
                            let program_length_bytes: [u8; 4] = data[1..=4].try_into().unwrap();
                            let expected_program_length = u32::from_be_bytes(program_length_bytes);

                            let worker_index_bytes: [u8; 2] = data[5..=6].try_into().unwrap();
                            let worker_index = u16::from_be_bytes(worker_index_bytes);

                            let job_id: [u8; 64] = data[7..=7 + 63].try_into().unwrap();
                            let job_id_str = String::from_utf8_lossy(&job_id);

                            let program_bytes = &data[7 + 63 + 1..];

                            let actual_len = program_bytes.len();
                            if actual_len != expected_program_length as usize {
                                println!("{program_bytes:?}");
                                panic!("Invalid program length received: {actual_len} is not expected {expected_program_length}")
                            }

                            let tx_body = &[
                                &[CODE_STARTED_JOB],
                                u16_into_bytes(worker_index).as_slice(),
                                job_id.as_slice(),
                            ]
                            .concat();

                            println!("==========> received job start: id=`{job_id_str}` for worker index=`{worker_index}` program_bytes={program_bytes:?}");

                            *worker_recv.lock().unwrap() = Some(worker_index);

                            tx_1.send(OwnedMessage::Binary(tx_body.to_owned())).unwrap();
                        }
                        CODE_PONG => {
                            println!("[RECV] PONG")
                        }
                        other => {
                            println!("[RECV]: Unkown message: {other:?}")
                        }
                    }
                }
                _ => println!("[RECV] other: {:?}", message),
            }
        }
    });

    let tx_send_loop = tx.clone();
    let init_bool = init_bool_src.clone();

    thread::spawn(move || {
        let mut iter_count = 0;

        loop {
            if !*init_bool.lock().unwrap() {
                thread::sleep(Duration::from_secs(1));
                continue;
            }

            tx_send_loop
                .send(OwnedMessage::Binary(vec![CODE_PING]))
                .unwrap();

            // TODO: configure this.
            thread::sleep(Duration::from_secs(3));

            // TODO: do logging!
            //
            iter_count += 1;

            if iter_count != 1{
                continue;
            }

            let Some(worker_idx) = *worker_arc.lock().unwrap() else {
                println!("~~~~~~~~~~~~~~~~~~~~~~ No worker index for progres logging");
                continue;
            };

            println!("~~~~~~~~~~~~ Progress log ~~~~~~~~~~~~~");

            let log_kind = 6; // progress

            let content = "Hello World Log!";
            let content_length = content.len();

            let tx_body = &[
                &[CODE_JOB_LOG],
                u16_into_bytes(log_kind).as_slice(),
                u16_into_bytes(worker_idx).as_slice(),
                u16_into_bytes(content_length as u16).as_slice(),
                content.as_bytes(),
            ]
            .concat();

            tx_send_loop
                .send(OwnedMessage::Binary(tx_body.to_owned()))
                .unwrap();

            iter_count = 0;
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
                panic!("received close");
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
                panic!("{e:?}")
            }
        }
    }

    println!("Waiting for child threads to exit");

    let _ = send_loop.join();
    let _ = receive_loop.join();

    println!("Exited");
}
