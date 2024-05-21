use std::io::{self, Read};

use crate::module::LEB128Ext;

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
pub struct FunctionSectionEntry {
    pub(crate) signature_index: usize,
}

pub fn parse_function_section<R: Read>(reader: &mut R) -> io::Result<Box<[FunctionSectionEntry]>> {
    let functions_len = reader.read_u32_leb()?;
    let mut function_entries = Vec::new();
    function_entries.reserve_exact(functions_len as usize);

    for _ in 0..functions_len {
        function_entries.push(FunctionSectionEntry {
            signature_index: reader.read_u32_leb()? as usize,
        });
    }

    Ok(function_entries.into_boxed_slice())
}
