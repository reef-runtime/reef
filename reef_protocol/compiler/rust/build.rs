macro_rules! p {
    ($($tokens: tt)*) => {
        println!("cargo:warning={}", format!($($tokens)*))
    }
}

fn main() {
    println!("cargo::rerun-if-changed=../schema/*.capnp");

    p!("Generating CAPNP code...");

    // retarted Go bullshit
    std::process::Command::new("make")
        .arg("build")
        .current_dir(std::env::current_dir().unwrap().parent().unwrap())
        .status()
        .unwrap();

    capnpc::CompilerCommand::new()
        .src_prefix("../schema/")
        .import_path("../go-capnp/std/")
        .file("../schema/compiler.capnp")
        .run()
        .expect("schema compiler command");

    p!("Generated Rust files in");
}
