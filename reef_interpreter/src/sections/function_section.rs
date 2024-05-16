use std::io::{self, Read};

use crate::leb128::LEB128Ext;

#[derive(Debug)]
pub struct FunctionSectionEntry {
    signature_index: usize,
}

pub fn parse_function_section<R: Read>(reader: &mut R) -> io::Result<Box<[FunctionSectionEntry]>> {
    let functions_len = reader.read_u32_leb()?;
    let mut function_entries = Vec::with_capacity(functions_len as usize);

    for _ in 0..functions_len {
        function_entries.push(FunctionSectionEntry {
            signature_index: reader.read_u32_leb()? as usize,
        });
    }

    Ok(function_entries.into_boxed_slice())
}
