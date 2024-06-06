//! Parser that translates [`wasmparser`](https://docs.rs/wasmparser) types to types used by this crate.

use alloc::{string::ToString, vec::Vec};

mod conversion;
pub(crate) mod error;
pub(crate) mod module;
mod visit;

use crate::types::{Module, WasmFunction};
use error::{ParseError, Result};
use module::ModuleReader;
use wasmparser::{Validator, WasmFeaturesInflated};

/// A WebAssembly parser
#[derive(Default, Debug)]
pub(crate) struct Parser {}

impl Parser {
    fn create_validator() -> Validator {
        let features = WasmFeaturesInflated {
            bulk_memory: true,
            floats: true,
            multi_value: true,
            mutable_global: true,
            reference_types: true,
            sign_extension: true,
            saturating_float_to_int: true,

            function_references: false,
            component_model: false,
            component_model_nested_names: false,
            component_model_values: false,
            exceptions: false,
            extended_const: false,
            gc: false,
            memory64: false,
            memory_control: false,
            relaxed_simd: false,
            simd: false,
            tail_call: false,
            threads: false,
            multi_memory: false, // should be working mostly
            custom_page_sizes: false,
            shared_everything_threads: false,
        };
        Validator::new_with_features(features.into())
    }

    /// Parse a [`Module`] from bytes
    pub(crate) fn parse_module_bytes(wasm: impl AsRef<[u8]>) -> Result<Module> {
        let wasm = wasm.as_ref();
        let mut validator = Self::create_validator();
        let mut reader = ModuleReader::new();

        for payload in wasmparser::Parser::new(0).parse_all(wasm) {
            reader.process_payload(payload?, &mut validator)?;
        }

        if !reader.end_reached {
            return Err(ParseError::EndNotReached);
        }

        reader.try_into()
    }
}

impl TryFrom<ModuleReader> for Module {
    type Error = ParseError;

    fn try_from(reader: ModuleReader) -> Result<Self> {
        if !reader.end_reached {
            return Err(ParseError::EndNotReached);
        }

        let code_type_addrs = reader.code_type_addrs;
        let local_function_count = reader.code.len();

        if code_type_addrs.len() != local_function_count {
            return Err(ParseError::Other("Code and code type address count mismatch".to_string()));
        }

        let funcs = reader
            .code
            .into_iter()
            .zip(code_type_addrs)
            .map(|((instructions, locals), ty_idx)| WasmFunction {
                instructions,
                locals,
                ty: reader.func_types.get(ty_idx as usize).expect("No func type for func, this is a bug").clone(),
            })
            .collect::<Vec<_>>();

        let globals = reader.globals;
        let table_types = reader.table_types;

        Ok(Module {
            funcs: funcs.into_boxed_slice(),
            func_types: reader.func_types.into_boxed_slice(),
            globals: globals.into_boxed_slice(),
            table_types: table_types.into_boxed_slice(),
            imports: reader.imports.into_boxed_slice(),
            start_func: reader.start_func,
            data: reader.data.into_boxed_slice(),
            exports: reader.exports.into_boxed_slice(),
            elements: reader.elements.into_boxed_slice(),
            memory_types: reader.memory_types.into_boxed_slice(),
        })
    }
}
