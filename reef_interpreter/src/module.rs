use crate::{error::Result, parser::Parser, types::Module};

/// Parse a module from bytes. Requires `parser` feature.
pub fn parse_bytes(wasm: &[u8]) -> Result<Module> {
    let data = Parser::parse_module_bytes(wasm)?;
    Ok(data)
}
