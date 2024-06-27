use std::fmt::Display;
use std::path::PathBuf;
use std::process::Command;
use std::str::FromStr;
use std::{fs, io};

use anyhow::Result;
use capnp::capability::Promise;
use capnp_rpc::pry;
use sha2::{Digest, Sha256};

use reef_protocol_compiler::compiler_capnp::{self, compiler};

const OUTPUT_FILE: &str = "output.wasm";

#[derive(Debug, Clone, Copy, Hash)]
enum Language {
    C,
    Rust,
}

impl From<&Language> for PathBuf {
    fn from(value: &Language) -> Self {
        match value {
            Language::C => PathBuf::from_str("c").unwrap(),
            Language::Rust => PathBuf::from_str("rust").unwrap(),
        }
    }
}

impl Display for Language {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Language::C => write!(f, "c"),
            Language::Rust => write!(f, "rust"),
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
    Compiler(String),
    Other(String),
    Io(io::Error),
}

impl From<io::Error> for Error {
    fn from(value: io::Error) -> Self {
        Self::Io(value)
    }
}

impl Compiler {
    fn compile_inner(&self, file_buf: &str, language: Language) -> Result<Vec<u8>, Error> {
        // Ensure template exists
        let mut template_path = self.lang_templates.clone();
        template_path.push(PathBuf::from(&language));

        dbg!(&template_path);

        if !template_path.exists() {
            return Err(Error::Other(format!("Template source path for language '{language}' does not exist")));
        }

        // Calculate Hash
        let mut hasher = Sha256::new();
        hasher.update(file_buf.as_bytes());
        hasher.update(&language.to_string());
        let hash = hex::encode(hasher.finalize());

        println!("New build dir '{}/{hash}' for Job with Lang '{language}'", self.build_path.display());

        // Create dirs
        fs::create_dir_all(&self.build_path)?;

        let mut job_path = self.build_path.clone();
        job_path.push(&hash);

        if fs::remove_dir_all(&job_path).is_ok() {
            println!("Cleaned up compilation context at '{}'", job_path.display());
        }

        println!("Copying '{}' to '{}'...", template_path.display(), job_path.display());
        copy_dir::copy_dir(&template_path, &job_path)?;

        let mut input_file_path = job_path.clone();
        input_file_path.push(format!("input.{}", language.file_ending()));

        println!("Writing source file...");
        fs::write(input_file_path, file_buf)?;

        let mut cmd = Command::new("make");
        let cmd_args = cmd
            .arg(format!("HASH={hash}"))
            .arg(format!("OUT_FILE={OUTPUT_FILE}"))
            .arg("-C")
            .arg(&job_path)
            .arg("build");

        println!("Running command {cmd_args:?}...");

        let output = cmd_args.output()?;

        if !output.status.success() {
            let output = String::from_utf8_lossy(&output.stderr);
            println!("failed to invoke compiler: {output}");
            return Err(Error::Compiler(output.into_owned()));
        }

        let mut output_path = job_path.clone();
        output_path.push(OUTPUT_FILE);

        println!("Reading output file from '{}'...", output_path.display());

        let artifact = fs::read(output_path)?;

        // Only cleanup if required.
        if self.no_cleanup {
            println!("[warning] not performing cleanup...");
        } else {
            fs::remove_dir_all(&job_path)?;
        }

        Ok(artifact)
    }
}

#[derive(Debug, Clone, Hash)]
pub(crate) struct Compiler {
    pub(crate) build_path: PathBuf,
    pub(crate) lang_templates: PathBuf,
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
                Error::Compiler(err) => results.get().init_response().set_compiler_error(err),
                Error::Io(err) => results.get().init_response().set_system_error(err.to_string()),
                Error::Other(err) => results.get().init_response().set_system_error(err),
            },
        }

        Promise::ok(())
    }
}
