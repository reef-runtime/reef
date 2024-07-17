use std::fmt::Display;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::str::FromStr;
use std::{fs, io};

use anyhow::Result;
use capnp::capability::Promise;
use capnp_rpc::pry;
use sha2::{Digest, Sha256};

use reef_protocol_compiler::compiler_capnp::{self, compiler};

const OUTPUT_FILE: &str = "./output.wasm";

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

        if !template_path.exists() {
            return Err(Error::Other(format!("Template source path for language '{language}' does not exist")));
        }

        // Calculate Hash
        let mut hasher = Sha256::new();
        hasher.update(file_buf.as_bytes());
        hasher.update(language.to_string());
        let hash = hex::encode(hasher.finalize());

        let mut job_path = self.build_path.clone();
        job_path.push(&hash);

        let res = self.compile_fallibe(file_buf, language, &hash, &template_path, &job_path);

        // Only cleanup if required.
        if self.no_cleanup {
            println!("[warning] not performing cleanup...");
        } else {
            fs::remove_dir_all(&job_path)?;
        }

        res
    }

    fn compile_fallibe(
        &self,
        file_buf: &str,
        language: Language,
        hash: &str,
        template_path: &Path,
        job_path: &Path,
    ) -> Result<Vec<u8>, Error> {
        println!("==> New build '{}/{hash}' for Job with Lang '{language}'", self.build_path.display());

        // Create dirs
        fs::create_dir_all(job_path)?;

        if fs::remove_dir_all(job_path).is_ok() {
            println!("Cleaned up compilation context at '{}'", job_path.display());
        }

        // Own copy implementation because we need to fix write perms while copying
        println!("Copying '{}' to '{}'...", template_path.display(), job_path.display());
        for entry in walkdir::WalkDir::new(template_path).into_iter().filter_map(|e| e.ok()) {
            let relative_path = match entry.path().strip_prefix(template_path) {
                Ok(rp) => rp,
                Err(_) => panic!("strip_prefix failed. this is a probably a bug."),
            };

            let mut target_path = job_path.to_owned();
            target_path.push(relative_path);

            let entry_metadata = entry.metadata().map_err(io::Error::other)?;

            if entry_metadata.is_dir() {
                fs::create_dir(&target_path)?;
            } else {
                fs::copy(entry.path(), &target_path)?;
            }

            // this is intended for container use where there is only one user anyways
            let mut perms = fs::metadata(&target_path)?.permissions();
            #[allow(clippy::permissions_set_readonly_false)]
            perms.set_readonly(false);
            fs::set_permissions(&target_path, perms)?;
        }

        let mut input_file_path = job_path.to_owned();
        input_file_path.push(format!("input.{}", language.file_ending()));

        println!("Writing source file '{}'", input_file_path.display());
        fs::write(input_file_path, file_buf)?;

        let mut cmd = Command::new("make");
        let cmd_args =
            cmd.arg(format!("HASH={hash}")).arg(format!("OUT_FILE={OUTPUT_FILE}")).arg("-C").arg(job_path).arg("build");

        println!("Running command {cmd_args:?}...");

        let output = cmd_args.output()?;

        if !output.status.success() {
            let output = String::from_utf8_lossy(&output.stderr);
            println!("failed to invoke compiler: {output}");
            return Err(Error::Compiler(output.into_owned()));
        }

        let mut output_path = job_path.to_owned();
        output_path.push(OUTPUT_FILE);

        println!("==> Reading output file from '{}'...", output_path.display());

        Ok(fs::read(output_path)?)
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

            Err(e) => {
                println!("Error: {e:?}");
                match e {
                    Error::Compiler(err) => results.get().init_response().set_compiler_error(err),
                    Error::Io(err) => results.get().init_response().set_system_error(err.to_string()),
                    Error::Other(err) => results.get().init_response().set_system_error(err),
                }
            }
        }

        Promise::ok(())
    }
}
