use std::io::{self, Cursor, Error};

use byteorder::{ReadBytesExt, LE};

mod leb128;
use leb128::LEB128Ext;

#[derive(Debug, Default)]
pub struct Module {
    type_section: (),
    import_section: (),
    function_section: (),
    table_section: (),
    linear_memory_section: (),
    global_section: (),
    export_section: (),
    start_section: (),
    code_section: (),
    data_section: (),
}

const WASM_MAGIC: &[u8; 4] = b"\0asm";

impl Module {
    pub fn parse(data: &[u8]) -> io::Result<Self> {
        let mut data = Cursor::new(data);
        let magic = data.read_u32::<LE>()?;
        if magic != u32::from_le_bytes(*WASM_MAGIC) {
            return Err(Error::other("Invalid Magic Bytes"));
        }

        Ok(Self {
            ..Default::default()
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn get_wat_as_wasm(name: &str) -> Vec<u8> {
        let mut wat_file = std::path::PathBuf::from(env!("CARGO_MANIFEST_DIR"));
        wat_file.push("tests/wat");
        wat_file.push(name);
        let mut wasm_file = wat_file.clone();
        wat_file.set_extension("wat");
        wasm_file.set_extension("wasm");
        std::process::Command::new("wat2wasm")
            .arg(wat_file)
            .arg("-o")
            .arg(&wasm_file)
            .status()
            .expect("wat2wasm command failed to execute");

        std::fs::read(wasm_file).expect("Failed to read wasm")
    }

    #[test]
    fn minimal_module() {
        let module =
            Module::parse(&get_wat_as_wasm("minimal_module")).expect("Failed to parse Wasm");
    }
}
