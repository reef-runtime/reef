use anyhow::{Context, Result};
use futures::AsyncReadExt;
use std::fmt::Display;
use std::hash::{DefaultHasher, Hash, Hasher};
use std::net::SocketAddr;
use std::path::PathBuf;
use std::process::Command;
use std::str::FromStr;
use std::{fs, io};

use capnp::capability::Promise;
use capnp_rpc::{pry, rpc_twoparty_capnp, twoparty, RpcSystem};

use reef_protocol_compiler::compiler_capnp::compiler;
use reef_protocol_compiler::compiler_capnp::{self};

const BUILD_PATH: &str = "./test/";
const SKELETON_PATH: &str = "./skeletons/";

struct CompilerManager {
    build_path: PathBuf,
    skeleton_path: PathBuf,
}

#[derive(Hash)]
enum Language {
    C,
    Rust,
}

impl From<&Language> for PathBuf {
    fn from(value: &Language) -> Self {
        match value {
            Language::C => PathBuf::from_str("c"),
            Language::Rust => PathBuf::from_str("rust"),
        }
        .expect("always valid, this is static")
    }
}

impl Display for Language {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Language::C => write!(f, "C"),
            Language::Rust => write!(f, "Rust"),
        }
    }
}

impl Language {
    fn file_ending(&self) -> &str {
        match self {
            Self::C => "c",
            Self::Rust => "rs",
        }
    }
}

#[derive(Debug)]
pub enum CError {
    CompilerError(String),
    SystemError(String),
}

impl From<io::Error> for CError {
    fn from(value: io::Error) -> Self {
        Self::SystemError(value.to_string())
    }
}

//
// Skeleton directory -> Build directory / <hash of the file>
//

impl CompilerManager {
    fn new(main_path: &str, skeleton_path: &str) -> Self {
        Self {
            build_path: main_path.into(),
            skeleton_path: skeleton_path.into(),
        }
    }

    fn compile(&self, file_buf: &str, language: Language) -> Result<Vec<u8>, CError> {
        let mut hasher = DefaultHasher::new();
        file_buf.hash(&mut hasher);
        language.hash(&mut hasher);

        let hash = format!("{:x}", hasher.finish());

        let mut wasm_artifact_name = hash.clone();
        wasm_artifact_name.push_str(".wasm");

        println!("Compiling {wasm_artifact_name}...");

        println!("creating build directory...");
        let mut this_compilation_path = self.build_path.clone();
        this_compilation_path.push(hash.as_str());

        match fs::create_dir_all(&this_compilation_path) {
            Ok(_) => (),
            Err(e) => {
                return Err(CError::SystemError(format!(
                    "failed to create build directory: {e}"
                )))
            }
        };

        let mut skeleton_source_path = self.skeleton_path.clone();
        skeleton_source_path.push(PathBuf::from(&language));

        if !skeleton_source_path.exists() {
            return Err(CError::SystemError(format!(
                "skeleton source path for language {language} does not exist"
            )));
        }

        println!("copying {skeleton_source_path:?} to {this_compilation_path:?}...");

        if !this_compilation_path.exists() {
            match copy_dir::copy_dir(&skeleton_source_path, &this_compilation_path) {
                Ok(_) => (),
                Err(e) => return Err(CError::SystemError(format!("failed to copy skeleton: {e}"))),
            }
        }

        let mut source_file = this_compilation_path.clone();
        source_file.push(format!(
            "input.{file_ending}",
            file_ending = language.file_ending()
        ));

        println!("writing source file...");
        match fs::write(source_file, file_buf) {
            Ok(_) => (),
            Err(e) => {
                return Err(CError::SystemError(format!(
                    "failed to write to source file: {e}"
                )))
            }
        };

        let mut build_path = self.build_path.clone();
        build_path.push("build");

        let mut cmd = Command::new("make");
        let cmd_args = cmd
            .arg(format!("HASH={}", hash))
            .arg("-C")
            .arg(build_path)
            .arg("build");

        println!("running command {cmd_args:?}");

        let output = match cmd_args.output() {
            Ok(output) => output,
            Err(e) => {
                return Err(CError::SystemError(format!(
                    "failed to invoke compiler: {e}"
                )))
            }
        };

        if !output.status.success() {
            return Err(CError::CompilerError(
                String::from_utf8_lossy(&output.stderr).into_owned(),
            ));
        }

        println!("reading {this_compilation_path:?}...");

        let data = match fs::read(this_compilation_path.as_path()) {
            Ok(data) => data,
            Err(e) => {
                return Err(CError::SystemError(format!(
                    "failed to read output artifact: {e}"
                )))
            }
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

        let manager = CompilerManager::new(BUILD_PATH, SKELETON_PATH);
        let compiler_res = manager.compile(program_src, language);

        match compiler_res {
            Ok(buf) => results
                .get()
                .init_response()
                .set_file_content(buf.as_slice()),

            Err(e) => match e {
                CError::CompilerError(err) => results.get().init_response().set_compiler_error(err),

                CError::SystemError(err) => results
                    .get()
                    .init_response()
                    .set_system_error(err.to_string()),
            },
        }

        Promise::ok(())
    }
}

pub async fn run_server_main(socket: SocketAddr) -> Result<()> {
    tokio::task::LocalSet::new()
        .run_until(async move {
            let listener = tokio::net::TcpListener::bind(&socket)
                .await
                .with_context(|| "failed to bind to socket")?;
            let compiler: compiler::Client = capnp_rpc::new_client(Compiler);

            loop {
                let (stream, _) = listener
                    .accept()
                    .await
                    .with_context(|| "failed to listen")?;
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
