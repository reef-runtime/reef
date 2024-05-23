use std::env::args;

fn main() {
    let wasm_data = std::fs::read(args().nth(1).expect("Please provide Wasm file."))
        .expect("Failed to read wasm file.");
    let module = reef_interpreter::module::Module::parse(&mut std::io::Cursor::new(wasm_data))
        .expect("Failed to parse Wasm file.");

    println!("{module:#?}");

    // build execution context
    let mut ctx = reef_interpreter::execution::ExecutionContext::start(
        module,
        &args().nth(2).expect("Please provide function to execute"),
        (),
    )
    .expect("Failed to start execution");

    dbg!(&ctx);

    loop {
        println!("NEXT STEP");
        let result = ctx.step();

        println!("RESULT");
        dbg!(&result);
        println!("STEP DONE");

        match result {
            Ok(result) => {
                if matches!(result, Some(_)) {
                    break;
                }
            }
            Err(err) => {
                println!("ERROR: {err}");
                break;
            }
        }
    }
}
