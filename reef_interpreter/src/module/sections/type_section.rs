use std::io::{self, Error, Read};

use byteorder::ReadBytesExt;
use num_enum::{TryFromPrimitive, TryFromPrimitiveError};

use crate::{module::LEB128Ext, ValueType};

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub enum TypeSectionEntry {
    Function {
        params: Box<[ValueType]>,
        returns: Box<[ValueType]>,
    },
}

pub fn parse_type_section<R: Read>(reader: &mut R) -> io::Result<Box<[TypeSectionEntry]>> {
    let types_len = reader.read_u32_leb()?;
    let mut type_entries = Vec::new();
    type_entries.reserve_exact(types_len as usize);

    for _ in 0..types_len {
        let type_form = reader.read_u8()?;
        let type_entry = match type_form {
            0x60 => TypeSectionEntry::Function {
                params: parse_value_type_array(reader)?,
                returns: parse_value_type_array(reader)?,
            },
            _ => {
                return Err(Error::other(format!(
                    "Unknown Type scetion entry form 0x{type_form:X}."
                )));
            }
        };
        type_entries.push(type_entry);
    }

    Ok(type_entries.into_boxed_slice())
}

pub fn parse_value_type_array<R: Read>(reader: &mut R) -> io::Result<Box<[ValueType]>> {
    let params_len = reader.read_u32_leb()?;
    let mut params = vec![0; params_len as usize];
    reader.read_exact(&mut params)?;
    params
        .into_iter()
        .map(ValueType::try_from_primitive)
        .collect::<Result<Box<[ValueType]>, TryFromPrimitiveError<ValueType>>>()
        .map_err(io::Error::other)
}