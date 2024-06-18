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

const OUTPUT_FILE: &str = "output.wasm";

#[derive(Debug, Clone, Copy, Hash)]
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
pub enum Error {
    CompilerError(String),
    Other(String),
    Io(io::Error),
}

impl From<io::Error> for Error {
    fn from(value: io::Error) -> Self {
        Self::Io(value)
    }
}

//
// Skeleton directory -> Build directory / <hash of the file>
//

impl Compiler {
    fn compile_inner(&self, file_buf: &str, language: Language) -> Result<Vec<u8>, Error> {
        let mut hasher = DefaultHasher::new();
        file_buf.hash(&mut hasher);
        language.hash(&mut hasher);

        let hash = format!("{:x}", hasher.finish());

        println!("creating build directory...");

        let root_build_path = self.build_path.clone();

        fs::create_dir_all(&root_build_path)?;

        let mut skeleton_source_path = self.skeleton_path.clone();
        skeleton_source_path.push(PathBuf::from(&language));

        if !skeleton_source_path.exists() {
            return Err(Error::Other(format!("skeleton source path for language {language} does not exist")));
        }

        let mut current_compilation_context = root_build_path.clone();
        current_compilation_context.push(&hash);

        if fs::remove_dir_all(&current_compilation_context).is_ok() {
            println!("cleaned up compilation context at {:?}", &current_compilation_context);
        }

        println!("copying {skeleton_source_path:?} to {current_compilation_context:?}...");

        if !current_compilation_context.exists() {
            copy_dir::copy_dir(&skeleton_source_path, &current_compilation_context)?;
        }

        let mut source_file = current_compilation_context.clone();
        source_file.push(format!("input.{file_ending}", file_ending = language.file_ending()));

        println!("writing source file...");
        fs::write(source_file, file_buf)?;

        let mut cmd = Command::new("make");
        let cmd_args = cmd
            .arg(format!("HASH={hash}"))
            .arg(format!("OUT_FILE={OUTPUT_FILE}"))
            .arg("-C")
            .arg(&current_compilation_context)
            .arg("build");

        println!("running command {cmd_args:?}...");

        let output = cmd_args.output()?;

        if !output.status.success() {
            let output = String::from_utf8_lossy(&output.stderr);
            println!("failed to invoke compiler: {output}");
            return Err(Error::CompilerError(output.into_owned()));
        }

        let mut output_path = current_compilation_context.clone();
        output_path.push(OUTPUT_FILE);

        println!("reading output file from {output_path:?}...");

        let data = fs::read(output_path)?;

        // Skip cleanup if required.
        if self.no_cleanup {
            println!("[warning] not performing cleanup...");
            return Ok(data);
        }

        fs::remove_dir_all(&current_compilation_context)?;

        Ok(data)
    }
}

#[derive(Debug, Clone, Hash)]
pub(crate) struct Compiler {
    pub(crate) build_path: PathBuf,
    pub(crate) skeleton_path: PathBuf,
    pub(crate) no_cleanup: bool,
}

impl compiler::Server for Compiler {
    fn compile(
        &mut self,
        params: compiler::CompileParams,
        mut results: compiler::CompileResults,
    ) -> Promise<(), capnp::Error> {
        let program_src = pry!(pry!(pry!(params.get()).get_program_src()).to_str());
        let language = match pry!(pry!(params.get()).get_language()) {
            compiler_capnp::Language::C => Language::C,
            compiler_capnp::Language::Rust => Language::Rust,
        };

        let compiler_res = self.compile_inner(program_src, language);

        match compiler_res {
            Ok(buf) => results.get().init_response().set_file_content(buf.as_slice()),

            Err(e) => match e {
                Error::CompilerError(err) => results.get().init_response().set_compiler_error(err),
                Error::Io(err) => results.get().init_response().set_system_error(err.to_string()),
                Error::Other(err) => results.get().init_response().set_system_error(err),
            },
        }

        Promise::ok(())
    }
}

pub async fn run_server_main(socket: SocketAddr, compiler: Compiler) -> Result<()> {
    tokio::task::LocalSet::new()
        .run_until(async move {
            let listener = tokio::net::TcpListener::bind(&socket).await.with_context(|| "failed to bind to socket")?;

            let compiler: compiler::Client = capnp_rpc::new_client(compiler);

            loop {
                let (stream, _) = listener.accept().await.with_context(|| "failed to listen")?;
                stream.set_nodelay(true)?;
                let (reader, writer) = tokio_util::compat::TokioAsyncReadCompatExt::compat(stream).split();
                let network =
                    twoparty::VatNetwork::new(reader, writer, rpc_twoparty_capnp::Side::Server, Default::default());

                let rpc_system = RpcSystem::new(Box::new(network), Some(compiler.clone().client));
                tokio::task::spawn_local(rpc_system);
            }
        })
        .await
}
