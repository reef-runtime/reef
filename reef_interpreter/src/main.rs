fn main() {
    let wasm_data = std::fs::read(std::env::args().nth(1).expect("Please provide Wasm file."))
        .expect("Failed to read wasm file.");

    let module = reef_interpreter::Module::parse(&wasm_data).expect("Failed to parse Wasm file.");
    println!("{module:#?}");
}
