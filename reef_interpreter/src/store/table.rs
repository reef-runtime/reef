use alloc::{vec, vec::Vec};

use crate::error::{Error, Result, Trap};
use crate::types::{
    value::{ValType, WasmValue},
    Addr, TableAddr, TableType,
};
use crate::unlikely;

const MAX_TABLE_SIZE: u32 = 10000000;

/// A WebAssembly Table Instance
///
/// See <https://webassembly.github.io/spec/core/exec/runtime.html#table-instances>
#[derive(Debug)]
pub(crate) struct TableInstance {
    pub(crate) elements: Vec<TableElement>,
    pub(crate) kind: TableType,
}

impl TableInstance {
    pub(crate) fn new(kind: TableType) -> Self {
        Self { elements: vec![TableElement::Uninitialized; kind.size_initial as usize], kind }
    }

    pub(crate) fn get_wasm_val(&self, addr: TableAddr) -> Result<WasmValue> {
        let val = self.get(addr)?.addr();

        Ok(match self.kind.element_type {
            ValType::RefFunc => val.map(WasmValue::RefFunc).unwrap_or(WasmValue::RefNull(ValType::RefFunc)),
            ValType::RefExtern => val.map(WasmValue::RefExtern).unwrap_or(WasmValue::RefNull(ValType::RefExtern)),
            _ => Err(Error::UnsupportedFeature("non-ref table".into()))?,
        })
    }

    pub(crate) fn get(&self, addr: TableAddr) -> Result<&TableElement> {
        self.elements.get(addr as usize).ok_or_else(|| Error::Trap(Trap::UndefinedElement { index: addr as usize }))
    }

    pub(crate) fn set(&mut self, table_idx: TableAddr, value: Addr) -> Result<()> {
        self.grow_to_fit(table_idx as usize + 1)
            .map(|_| self.elements[table_idx as usize] = TableElement::Initialized(value))
    }

    pub(crate) fn grow_to_fit(&mut self, new_size: usize) -> Result<()> {
        if new_size > self.elements.len() {
            if unlikely(new_size > self.kind.size_max.unwrap_or(MAX_TABLE_SIZE) as usize) {
                return Err(Trap::TableOutOfBounds { offset: new_size, len: 1, max: self.elements.len() }.into());
            }

            self.elements.resize(new_size, TableElement::Uninitialized);
        }
        Ok(())
    }

    pub(crate) fn size(&self) -> i32 {
        self.elements.len() as i32
    }

    // Initialize the table with the given elements
    pub(crate) fn init_raw(&mut self, offset: i32, init: &[TableElement]) -> Result<()> {
        let offset = offset as usize;
        let end = offset
            .checked_add(init.len())
            .ok_or_else(|| Error::Trap(Trap::TableOutOfBounds { offset, len: init.len(), max: self.elements.len() }))?;

        if end > self.elements.len() || end < offset {
            return Err(Trap::TableOutOfBounds { offset, len: init.len(), max: self.elements.len() }.into());
        }

        self.elements[offset..end].copy_from_slice(init);
        Ok(())
    }

    // Initialize the table with the given elements (resolves function references)
    pub(crate) fn init(&mut self, offset: i32, init: &[TableElement]) -> Result<()> {
        let init = init.iter().map(|item| item.map(|addr| addr)).collect::<Vec<_>>();
        self.init_raw(offset, &init)
    }
}

#[derive(Debug, Clone, Copy)]
pub(crate) enum TableElement {
    Uninitialized,
    Initialized(TableAddr),
}

impl From<Option<Addr>> for TableElement {
    fn from(addr: Option<Addr>) -> Self {
        match addr {
            None => TableElement::Uninitialized,
            Some(addr) => TableElement::Initialized(addr),
        }
    }
}

impl TableElement {
    pub(crate) fn addr(&self) -> Option<Addr> {
        match self {
            TableElement::Uninitialized => None,
            TableElement::Initialized(addr) => Some(*addr),
        }
    }

    pub(crate) fn map<F: FnOnce(Addr) -> Addr>(self, f: F) -> Self {
        match self {
            TableElement::Uninitialized => TableElement::Uninitialized,
            TableElement::Initialized(addr) => TableElement::Initialized(f(addr)),
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    // Helper to create a dummy TableType
    fn dummy_table_type() -> TableType {
        TableType { element_type: ValType::RefFunc, size_initial: 10, size_max: Some(20) }
    }

    #[test]
    fn test_table_instance_creation() {
        let kind = dummy_table_type();
        let table_instance = TableInstance::new(kind.clone());
        assert_eq!(table_instance.size(), kind.size_initial as i32, "Table instance creation failed: size mismatch");
    }

    #[test]
    fn test_get_wasm_val() {
        let kind = dummy_table_type();
        let mut table_instance = TableInstance::new(kind);

        table_instance.set(0, 0).expect("Setting table element failed");

        match table_instance.get_wasm_val(0) {
            Ok(WasmValue::RefFunc(_)) => {}
            _ => panic!("get_wasm_val failed to return the correct WasmValue"),
        }

        match table_instance.get_wasm_val(999) {
            Err(Error::Trap(Trap::UndefinedElement { .. })) => {}
            _ => panic!("get_wasm_val failed to handle undefined element correctly"),
        }
    }

    #[test]
    fn test_set_and_get() {
        let kind = dummy_table_type();
        let mut table_instance = TableInstance::new(kind);

        let result = table_instance.set(0, 1);
        assert!(result.is_ok(), "Setting table element failed");

        let elem = table_instance.get(0);
        assert!(
            elem.is_ok() && matches!(elem.unwrap(), &TableElement::Initialized(1)),
            "Getting table element failed or returned incorrect value"
        );
    }

    #[test]
    fn test_table_grow_and_fit() {
        let kind = dummy_table_type();
        let mut table_instance = TableInstance::new(kind);

        let result = table_instance.set(15, 1);
        assert!(result.is_ok(), "Table grow on set failed");

        let size = table_instance.size();
        assert!(size >= 16, "Table did not grow to expected size");
    }

    #[test]
    fn test_table_init() {
        let kind = dummy_table_type();
        let mut table_instance = TableInstance::new(kind);

        let init_elements = vec![TableElement::Initialized(0); 5];
        let result = table_instance.init(0, &init_elements);

        assert!(result.is_ok(), "Initializing table with elements failed");

        for i in 0..5 {
            let elem = table_instance.get(i);
            assert!(
                elem.is_ok() && matches!(elem.unwrap(), &TableElement::Initialized(_)),
                "Element not initialized correctly at index {}",
                i
            );
        }
    }
}
