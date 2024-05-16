use std::io::{self, Cursor, Read, Seek};

use byteorder::ReadBytesExt;

use crate::leb128::LEB128Ext;
use crate::ExternalKind;

#[derive(Debug)]
pub struct CodeSectionEntry {
    locals: (),
    instructions: Box<[u8]>,
}

pub fn parse_code_section<R: Read>(reader: &mut R) -> io::Result<Box<[CodeSectionEntry]>> {
    let codes_len = reader.read_u32_leb()?;
    let mut code_entries = Vec::new();
    code_entries.reserve_exact(codes_len as usize);

    for _ in 0..codes_len {
        let function_len = dbg!(reader.read_u32_leb()?);
        let mut function_buf = vec![0; function_len as usize];
        reader.read_exact(&mut function_buf)?;
        let mut locals_cursor = Cursor::new(function_buf);

        let num_locals = dbg!(locals_cursor.read_u32_leb()?);
        for _ in 0..num_locals {
            todo!("TODO: locals not yet supported")
        }

        let code_len = function_len - locals_cursor.stream_position()? as u32;
        let mut code_buf = vec![0; code_len as usize];
        locals_cursor.read_exact(&mut code_buf)?;

        code_entries.push(CodeSectionEntry {
            locals: (),
            instructions: code_buf.into(),
        });
    }

    Ok(code_entries.into_boxed_slice())
}
