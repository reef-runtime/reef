use futures::AsyncReadExt;
use std::fs;
use std::hash::{DefaultHasher, Hash, Hasher};
use std::path::PathBuf;
use std::process::Command;

use capnp::capability::Promise;
use capnp_rpc::{pry, rpc_twoparty_capnp, twoparty, RpcSystem};

use reef_compiler::compiler_capnp;
use reef_compiler::compiler_capnp::compiler;

const MAIN_PATH: &str = "../test/";
const SKELETON_PATH: &str = "../skeletons/";

struct CompilerManager {
    main_path: PathBuf,
    skeleton_path: PathBuf,
}

enum Language {
    C,
    Rust,
}

#[derive(Debug)]
pub enum CError {
    CompilerError(String),
    FSError(std::io::Error),
}

impl CompilerManager {
    fn new(main_path: &str, skeleton_path: &str) -> Self {
        return Self {
            main_path: PathBuf::from(main_path),
            skeleton_path: PathBuf::from(skeleton_path),
        };
    }
    fn compile(&self, file_buf: &str, language: Language) -> Result<Vec<u8>, CError> {
        let mut hasher = DefaultHasher::new();
        file_buf.hash(&mut hasher);

        let hash = format!("{:x}", hasher.finish());

        let mut artifact_name = hash.clone();
        artifact_name.push_str(".wasm");

        // <main_path>/output/<hash>/
        let mut artifact_path = self.main_path.clone();
        artifact_path.push("output");
        artifact_path.push(hash.as_str());

        // <main_path>/src/<hash>/
        let mut src_path = self.main_path.clone();
        src_path.push("src");
        src_path.push(hash.as_str());

        if artifact_path.is_dir() {
            artifact_path.push(artifact_name);
            match fs::read(artifact_path.as_path()) {
                Ok(data) => return Ok(data),
                Err(e) => return Err(CError::FSError(e)),
            };
        }

        match fs::create_dir_all(&artifact_path) {
            Ok(_) => (),
            Err(e) => return Err(CError::FSError(e)),
        };

        match fs::create_dir_all(&src_path) {
            Ok(_) => (),
            Err(e) => return Err(CError::FSError(e)),
        };

        artifact_path.push(artifact_name);

        let mut src_name = hash.clone();
        match language {
            Language::C => src_name.push_str(".c"),
            Language::Rust => src_name.push_str(".rs"),
        }
        src_path.push(src_name);

        match fs::write(src_path.clone(), file_buf) {
            Ok(_) => (),
            Err(e) => return Err(CError::FSError(e)),
        };

        let mut skeleton = self.skeleton_path.clone();
        match language {
            Language::C => {
                skeleton.push("c");
            }
            Language::Rust => {
                skeleton.push("rust");
            }
        }

        let mut build_path = self.main_path.clone();
        build_path.push("build");

        match copy_dir::copy_dir(&skeleton, &build_path) {
            Ok(_) => (),
            Err(e) => return Err(CError::FSError(e)),
        }

        let output = match Command::new("make")
            .arg(format!("HASH={}", hash))
            .arg("-C")
            .arg(build_path)
            .arg("build")
            .output()
        {
            Ok(output) => output,
            Err(e) => return Err(CError::FSError(e)),
        };

        if !output.status.success() {
            return Err(CError::CompilerError(
                String::from_utf8_lossy(&output.stderr).into_owned(),
            ));
        }

        let data = match fs::read(artifact_path.as_path()) {
            Ok(data) => data,
            Err(e) => return Err(CError::FSError(e)),
        };

        Ok(data)
    }
}

struct Compiler;

impl compiler::Server for Compiler {
    fn compile(
        &mut self,
        params: compiler::CompileParams,
        mut results: compiler::CompileResults,
    ) -> Promise<(), ::capnp::Error> {
        let program_src = pry!(pry!(pry!(params.get()).get_program_src()).to_str());
        let language = match pry!(pry!(params.get()).get_language()) {
            compiler_capnp::Language::C => Language::C,
            compiler_capnp::Language::Rust => Language::Rust,
        };

        let manager = CompilerManager::new(MAIN_PATH, SKELETON_PATH);
        let compiler_res = manager.compile(program_src, language);

        match compiler_res {
            Ok(buf) => results
                .get()
                .init_response()
                .set_file_content(buf.as_slice()),

            Err(e) => match e {
                CError::CompilerError(err) => results.get().init_response().set_compiler_error(err),

                CError::FSError(err) => results
                    .get()
                    .init_response()
                    .set_file_system_error(err.to_string()),
            },
        }

        Promise::ok(())
    }
}

#[tokio::main(flavor = "current_thread")]
pub async fn main() -> Result<(), Box<dyn std::error::Error>> {
    use std::net::ToSocketAddrs;
    let args: Vec<String> = ::std::env::args().collect();
    if args.len() != 2 {
        println!("usage: {} ADDRESS[:PORT]", args[0]);
        return Ok(());
    }

    let addr = args[1]
        .to_socket_addrs()?
        .next()
        .expect("could not parse address");

    tokio::task::LocalSet::new()
        .run_until(async move {
            let listener = tokio::net::TcpListener::bind(&addr).await?;
            let compiler: compiler::Client = capnp_rpc::new_client(Compiler);

            loop {
                let (stream, _) = listener.accept().await?;
                stream.set_nodelay(true)?;
                let (reader, writer) =
                    tokio_util::compat::TokioAsyncReadCompatExt::compat(stream).split();
                let network = twoparty::VatNetwork::new(
                    reader,
                    writer,
                    rpc_twoparty_capnp::Side::Server,
                    Default::default(),
                );

                let rpc_system = RpcSystem::new(Box::new(network), Some(compiler.clone().client));
                tokio::task::spawn_local(rpc_system);
            }
        })
        .await
}
