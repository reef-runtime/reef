use std::io::{self, Read};

use byteorder::ReadBytesExt;

use crate::module::{ExternalKind, LEB128Ext};

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct ExportSectionEntry {
    pub(crate) export_name: Box<str>,
    pub(crate) export_kind: ExternalKind,
    pub(crate) export_index: usize,
}

pub fn parse_export_section<R: Read>(reader: &mut R) -> io::Result<Box<[ExportSectionEntry]>> {
    let exports_len = reader.read_u32_leb()?;
    let mut export_entries = Vec::new();
    export_entries.reserve_exact(exports_len as usize);

    for _ in 0..exports_len {
        let name_len = reader.read_u32_leb()?;
        let mut name_buf = vec![0; name_len as usize];
        reader.read_exact(&mut name_buf)?;
        let export_name = String::from_utf8(name_buf)
            .map_err(io::Error::other)?
            .into();

        let export_kind = reader.read_u8()?.try_into().map_err(io::Error::other)?;

        export_entries.push(ExportSectionEntry {
            export_name,
            export_kind,
            export_index: reader.read_u32_leb()? as usize,
        });
    }

    Ok(export_entries.into_boxed_slice())
}
