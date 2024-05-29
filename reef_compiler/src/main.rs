use std::fs;
use std::hash::{DefaultHasher, Hash, Hasher};
use std::path::PathBuf;
use std::process::Command;
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

        match copy_dir::copy_dir(skeleton, build_path) {
            Ok(_) => (),
            Err(e) => return Err(CError::FSError(e)),
        }

        let output = match Command::new("make")
            .arg(format!("HASH={}", hash))
            .arg("-C")
            .arg("./build")
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

fn test() {
    let m = CompilerManager::new("../test/", "../skeletons/");

    let content = m.compile("int main() {}", Language::C).unwrap();

    println!("{}", content.len());
}

fn main() {
    test();
}
