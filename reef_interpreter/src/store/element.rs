use alloc::vec::Vec;

use crate::store::table::TableElement;
use crate::types::ElementKind;

/// A WebAssembly Element Instance
///
/// See <https://webassembly.github.io/spec/core/exec/runtime.html#element-instances>
#[derive(Debug)]
pub(crate) struct ElementInstance {
    pub(crate) kind: ElementKind,
    pub(crate) items: Option<Vec<TableElement>>, // none is the element was dropped
}

impl ElementInstance {
    pub(crate) fn new(kind: ElementKind, items: Option<Vec<TableElement>>) -> Self {
        Self { kind, items }
    }
}
