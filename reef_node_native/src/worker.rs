use std::thread;

pub(crate) fn spawn_worker_thread() {
    // TODO: implement this.
    thread::spawn(|| loop {
        // socket.send(log_message(2, 42, "Hallo".as_bytes())).unwrap();
    });
}
