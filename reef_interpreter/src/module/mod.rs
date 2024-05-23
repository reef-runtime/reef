use std::io::{self, Error, Read};

use byteorder::{ReadBytesExt, LE};
use num_enum::TryFromPrimitive;

pub mod leb128;
use leb128::LEB128Ext;
pub mod instruction;
mod sections;

#[derive(Debug, Clone, PartialEq, Eq, Hash, Default)]
pub struct Module {
    pub(crate) type_section: Box<[sections::type_section::TypeSectionEntry]>,
    pub(crate) import_section: (),
    pub(crate) function_section: Box<[sections::function_section::FunctionSectionEntry]>,
    pub(crate) table_section: (),
    pub(crate) linear_memory_section: (),
    pub(crate) global_section: (),
    pub(crate) export_section: Box<[sections::export_section::ExportSectionEntry]>,
    pub(crate) start_section: (),
    pub(crate) element_section: (),
    pub(crate) code_section: Box<[sections::code_section::CodeSectionEntry]>,
    pub(crate) data_section: (),
}

const WASM_MAGIC: &[u8; 4] = b"\0asm";

impl Module {
    pub fn parse<R: Read>(reader: &mut R) -> io::Result<Self> {
        // let mut reader = Cursor::new(reader);
        let magic = reader.read_u32::<LE>()?;
        if magic != u32::from_le_bytes(*WASM_MAGIC) {
            return Err(Error::other("Invalid Magic Bytes"));
        }

        let version = reader.read_u32::<LE>()?;
        if version != 1 {
            return Err(Error::other(format!(
                "Unsupported Wasm binary version {version}."
            )));
        }

        let mut module = Module::default();

        // let mut prev_section_code = 0u8;
        loop {
            dbg!("NEXT SECTION");
            let mut section_code = [0];
            if reader.read(&mut section_code)? == 0 {
                // EOF
                break;
            }

            let section_code = section_code[0];
            // if section_code <= prev_section_code {
            //     return Err(Error::other(format!(
            //         "Section {section_code} is out of order."
            //     )));
            // }
            // prev_section_code = section_code;

            let _section_size = reader.read_u32_leb()?;
            match section_code {
                // Type section
                0x01 => {
                    module.type_section = sections::type_section::parse_type_section(reader)?;
                }
                0x03 => {
                    module.function_section =
                        sections::function_section::parse_function_section(reader)?;
                }
                0x07 => {
                    module.export_section = sections::export_section::parse_export_section(reader)?;
                }
                0x0A => {
                    module.code_section = sections::code_section::parse_code_section(reader)?;
                }
                _ => {
                    return Err(Error::other(format!(
                        "Invalid section code 0x{section_code:X}."
                    )));
                }
            }
        }

        Ok(module)
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default, TryFromPrimitive)]
#[repr(u8)]
pub enum ExternalKind {
    #[default]
    Function = 0x00,
    Table = 0x01,
    Memory = 0x02,
    Global = 0x03,
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    fn get_module(name: &str) -> Module {
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

        let bytes = std::fs::read(wasm_file).expect("Failed to read wasm");
        Module::parse(&mut Cursor::new(bytes)).expect("Failed to parse Wasm")
    }

    #[test]
    fn minimal_module() {
        let module = get_module("minimal_module");
        assert_eq!(module.type_section.len(), 0);
        assert_eq!(module.function_section.len(), 0);
        assert_eq!(module.export_section.len(), 0);
        assert_eq!(module.code_section.len(), 0);
    }

    #[test]
    fn function_end() {
        let module = get_module("function_end");
        assert_eq!(module.type_section.len(), 1);
        assert_eq!(module.function_section.len(), 1);
        assert_eq!(module.export_section.len(), 1);
        assert_eq!(module.code_section.len(), 1);

        assert_eq!(module.code_section[0].instructions.len(), 1);
    }

    #[test]
    fn function_nop() {
        let module = get_module("function_nop");
        assert_eq!(module.type_section.len(), 1);
        assert_eq!(module.function_section.len(), 1);
        assert_eq!(module.export_section.len(), 1);
        assert_eq!(module.code_section.len(), 1);

        assert_eq!(module.code_section[0].instructions.len(), 2);
    }

    #[test]
    fn block() {
        let module = get_module("block");
        assert_eq!(module.type_section.len(), 1);
        assert_eq!(module.function_section.len(), 1);
        assert_eq!(module.export_section.len(), 1);
        assert_eq!(module.code_section.len(), 1);

        assert_eq!(module.code_section[0].instructions.len(), 8);
    }
}
