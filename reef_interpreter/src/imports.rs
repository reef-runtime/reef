//! Types for resources that a Wasm module requires

use alloc::{
    boxed::Box,
    collections::BTreeMap,
    format,
    string::{String, ToString},
    vec::Vec,
};
use core::fmt::Debug;

use crate::error::{Error, LinkingError, Result};
use crate::func::{FromWasmValueTuple, IntoWasmValueTuple, ValTypesFromTuple};
use crate::reference::{MemoryRef, MemoryRefMut};
use crate::store::memory::MemoryInstance;
use crate::types::{
    value::WasmValue, ExternalKind, FuncAddr, GlobalAddr, GlobalType, Import, MemAddr, MemoryType, Module, TableAddr,
    TableType,
};
use crate::types::{FuncType, WasmFunction};
use crate::VecExt;

/// The internal representation of a function
#[derive(Debug)]
pub enum Function {
    /// A host function
    Host(HostFunction),

    /// A pointer to a WebAssembly function
    Wasm(WasmFunction),
}

impl Function {
    pub(crate) fn ty(&self) -> &FuncType {
        match self {
            Self::Host(f) => &f.ty,
            Self::Wasm(f) => &f.ty,
        }
    }
}

/// A host function
pub struct HostFunction {
    pub(crate) ty: FuncType,
    pub(crate) func: HostFuncInner,
}

impl HostFunction {
    /// Get the function's type
    pub fn ty(&self) -> &FuncType {
        &self.ty
    }

    /// Call the function
    pub fn call(&self, ctx: FuncContext<'_>, args: &[WasmValue]) -> Result<Vec<WasmValue>> {
        (self.func)(ctx, args)
    }
}

pub(crate) type HostFuncInner = Box<dyn Fn(FuncContext<'_>, &[WasmValue]) -> Result<Vec<WasmValue>>>;

/// The context of a host-function call
#[derive(Debug)]
pub struct FuncContext<'i> {
    pub(crate) module: &'i Module,
    pub(crate) memories: &'i mut Vec<MemoryInstance>,
}

impl FuncContext<'_> {
    /// Get a reference to the module instance
    pub fn module(&self) -> &crate::Module {
        self.module
    }

    /// Get a reference to an exported memory
    pub fn exported_memory(&self, name: &str) -> Result<MemoryRef<'_>> {
        Ok(MemoryRef { instance: self.memories.get_or_instance(self.exported_memory_addr(name)?, "memory")? })
    }

    /// Get a reference to an exported memory
    pub fn exported_memory_mut(&mut self, name: &str) -> Result<MemoryRefMut<'_>> {
        Ok(MemoryRefMut { instance: self.memories.get_mut_or_instance(self.exported_memory_addr(name)?, "memory")? })
    }

    fn exported_memory_addr(&self, name: &str) -> Result<u32> {
        let export = self
            .module
            .exports
            .iter()
            .find(|e| &*e.name == name)
            .ok_or_else(|| Error::Other(format!("Export not found: {}", name)))?;

        if export.kind != ExternalKind::Memory {
            return Err(Error::Other(format!("Export is not a memory: {}", name)));
        };

        Ok(export.index)
    }
}

impl Debug for HostFunction {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        f.debug_struct("HostFunction").field("ty", &self.ty).field("func", &"...").finish()
    }
}

#[derive(Debug)]
#[non_exhaustive]
/// An external value
pub enum Extern {
    /// A global value
    Global {
        /// The type of the global value.
        ty: GlobalType,
        /// The actual value of the global, encapsulated in `WasmValue`.
        val: WasmValue,
    },

    /// A table
    Table {
        /// Defines the type of the table, including its element type and limits.
        ty: TableType,
        /// The initial value of the table.
        init: WasmValue,
    },

    /// A memory
    Memory {
        /// Defines the type of the memory, including its limits and the type of its pages.
        ty: MemoryType,
    },

    /// A function
    Function(Option<Function>),
}

impl Extern {
    /// Create a new global import
    pub fn global(val: WasmValue, mutable: bool) -> Self {
        Self::Global { ty: GlobalType { ty: val.val_type(), mutable }, val }
    }

    /// Create a new table import
    pub fn table(ty: TableType, init: WasmValue) -> Self {
        Self::Table { ty, init }
    }

    /// Create a new memory import
    pub fn memory(ty: MemoryType) -> Self {
        Self::Memory { ty }
    }

    /// Create a new function import
    pub fn func(
        ty: &FuncType,
        func: impl Fn(FuncContext<'_>, &[WasmValue]) -> Result<Vec<WasmValue>> + 'static,
    ) -> Self {
        Self::Function(Some(Function::Host(HostFunction { func: Box::new(func), ty: ty.clone() })))
    }

    /// Create a new typed function import
    // TODO: currently, this is slower than `Extern::func` because of the type conversions.
    //       we should be able to optimize this and make it even faster than `Extern::func`.
    pub fn typed_func<P, R>(func: impl Fn(FuncContext<'_>, P) -> Result<R> + 'static) -> Self
    where
        P: FromWasmValueTuple + ValTypesFromTuple,
        R: IntoWasmValueTuple + ValTypesFromTuple + Debug,
    {
        let inner_func = move |ctx: FuncContext<'_>, args: &[WasmValue]| -> Result<Vec<WasmValue>> {
            let args = P::from_wasm_value_tuple(args)?;
            let result = func(ctx, args)?;
            Ok(result.into_wasm_value_tuple().to_vec())
        };

        let ty = FuncType { params: P::val_types(), results: R::val_types() };
        Self::Function(Some(Function::Host(HostFunction { func: Box::new(inner_func), ty })))
    }

    /// Get the kind of the external value
    pub fn kind(&self) -> ExternalKind {
        match self {
            Self::Global { .. } => ExternalKind::Global,
            Self::Table { .. } => ExternalKind::Table,
            Self::Memory { .. } => ExternalKind::Memory,
            Self::Function { .. } => ExternalKind::Func,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Eq, Ord, PartialOrd, Hash)]
/// Name of an import
pub struct ExternName {
    module: String,
    name: String,
}

impl From<&Import> for ExternName {
    fn from(import: &Import) -> Self {
        Self { module: import.module.to_string(), name: import.name.to_string() }
    }
}

#[derive(Debug, Default)]
/// Imports for a module instance
///
/// This is used to link a module instance to its imports
///
/// Note that module instance addresses for [`Imports::link_module`] can be obtained from [`crate::ModuleInstance::id`].
/// Now, the imports object can be passed to [`crate::ModuleInstance::instantiate`].

// #[derive(Clone)]
pub struct Imports {
    values: BTreeMap<ExternName, Extern>,
}

pub(crate) struct ResolvedImports {
    pub(crate) globals: Vec<GlobalAddr>,
    pub(crate) tables: Vec<TableAddr>,
    pub(crate) memories: Vec<MemAddr>,
    pub(crate) funcs: Vec<FuncAddr>,
}

impl ResolvedImports {
    pub(crate) fn new() -> Self {
        Self { globals: Vec::new(), tables: Vec::new(), memories: Vec::new(), funcs: Vec::new() }
    }
}

impl Imports {
    /// Create a new empty import set
    pub fn new() -> Self {
        Imports { values: BTreeMap::new() }
    }

    /// Merge two import sets
    pub fn merge(mut self, other: Self) -> Self {
        self.values.extend(other.values);
        self
    }

    /// Define an import
    pub fn define(&mut self, module: &str, name: &str, value: Extern) -> Result<&mut Self> {
        self.values.insert(ExternName { module: module.to_string(), name: name.to_string() }, value);
        Ok(self)
    }

    pub(crate) fn take(&mut self, import: &Import) -> Option<Extern> {
        let name = ExternName::from(import);
        self.values.remove(&name)
    }

    pub(crate) fn compare_types<T: Debug + PartialEq>(import: &Import, actual: &T, expected: &T) -> Result<()> {
        if expected != actual {
            return Err(LinkingError::incompatible_import_type(import).into());
        }
        Ok(())
    }

    pub(crate) fn compare_table_types(import: &Import, expected: &TableType, actual: &TableType) -> Result<()> {
        Self::compare_types(import, &actual.element_type, &expected.element_type)?;

        if actual.size_initial > expected.size_initial {
            return Err(LinkingError::incompatible_import_type(import).into());
        }

        match (expected.size_max, actual.size_max) {
            (None, Some(_)) => return Err(LinkingError::incompatible_import_type(import).into()),
            (Some(expected_max), Some(actual_max)) if actual_max < expected_max => {
                return Err(LinkingError::incompatible_import_type(import).into())
            }
            _ => {}
        }

        Ok(())
    }

    pub(crate) fn compare_memory_types(
        import: &Import,
        expected: &MemoryType,
        actual: &MemoryType,
        real_size: Option<usize>,
    ) -> Result<()> {
        Self::compare_types(import, &expected.arch, &actual.arch)?;

        if actual.page_count_initial > expected.page_count_initial
            && real_size.map_or(true, |size| actual.page_count_initial > size as u64)
        {
            return Err(LinkingError::incompatible_import_type(import).into());
        }

        if expected.page_count_max.is_none() && actual.page_count_max.is_some() {
            return Err(LinkingError::incompatible_import_type(import).into());
        }

        if let (Some(expected_max), Some(actual_max)) = (expected.page_count_max, actual.page_count_max) {
            if actual_max < expected_max {
                return Err(LinkingError::incompatible_import_type(import).into());
            }
        }

        Ok(())
    }

    // pub(crate) fn link(mut self, instance: &mut Instance) -> Result<ResolvedImports> {

    // }
}
