use alloc::{format, string::ToString};

use crate::error::{Error, Result};
use crate::runtime::RawWasmValue;
use crate::types::{value::WasmValue, GlobalType};
use crate::unlikely;

/// A WebAssembly Global Instance
///
/// See <https://webassembly.github.io/spec/core/exec/runtime.html#global-instances>
#[derive(Debug)]
pub(crate) struct GlobalInstance {
    pub(crate) value: RawWasmValue,
    pub(crate) ty: GlobalType,
}

impl GlobalInstance {
    pub(crate) fn new(ty: GlobalType, value: RawWasmValue) -> Self {
        Self { ty, value }
    }

    #[inline]
    pub(crate) fn get(&self) -> WasmValue {
        self.value.attach_type(self.ty.ty)
    }

    pub(crate) fn set(&mut self, val: WasmValue) -> Result<()> {
        if unlikely(val.val_type() != self.ty.ty) {
            return Err(Error::Other(format!(
                "global type mismatch: expected {:?}, got {:?}",
                self.ty.ty,
                val.val_type()
            )));
        }

        if unlikely(!self.ty.mutable) {
            return Err(Error::Other("global is immutable".to_string()));
        }

        self.value = val.into();
        Ok(())
    }
}
