use crate::runtime::RawWasmValue;

/// A WebAssembly Global Instance
///
/// See <https://webassembly.github.io/spec/core/exec/runtime.html#global-instances>
#[derive(Debug)]
pub(crate) struct GlobalInstance {
    pub(crate) value: RawWasmValue,
}

impl GlobalInstance {
    pub(crate) fn new(value: RawWasmValue) -> Self {
        Self { value }
    }
}
