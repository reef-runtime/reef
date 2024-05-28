use std::{sync::{mpsc, Arc}, thread};

use crate::comms;

enum WorkerMessage {
    Log,
}

type WorkerChannel = Arc<mpsc::Sender<WorkerMessage>>;

pub(crate) fn spawn_worker_thread(worker_index: u16) {
    // TODO: implement this.
    thread::spawn(move || loop {
        // socket.send(log_message(2, 42, "Hallo".as_bytes())).unwrap();
        let message = comms::log_message(42, worker_index, "Hallo an alle!".as_bytes());
    });
}
