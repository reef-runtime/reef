macro_rules! p {
    ($($tokens: tt)*) => {
        println!("cargo:warning={}", format!($($tokens)*))
    }
}

fn main() {
    println!("cargo::rerun-if-changed=../schema/*.capnp");

    p!("Generating CAPNP code...");

    capnpc::CompilerCommand::new()
        .src_prefix("../schema/")
        .file("../schema/message.capnp")
        .run()
        .expect("schema compiler command");

    p!("Generated Rust files in");
}
