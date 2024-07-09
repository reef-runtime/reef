pub fn run(dataset: &[u8]) -> impl Into<ReefResult> {
    let msg = "Hello World!";

    // Manual log invocation and println are the same for reef Rust.
    reef::reef_log(msg);

    // You can use format strings here.
    println!("{msg} {}", 2);
}
