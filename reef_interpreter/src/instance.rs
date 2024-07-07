use alloc::{format, string::ToString, vec::Vec};

use crate::error::{Error, LinkingError, Result, Trap};
use crate::exec::DeserializationState;
use crate::func::{FromWasmValueTuple, FuncHandle, FuncHandleTyped, IntoWasmValueTuple};
use crate::imports::{Extern, Function, Imports, ResolvedImports};
use crate::reference::{MemoryRef, MemoryRefMut};
use crate::runtime::{RawWasmValue, Stack};
use crate::store::{
    data::DataInstance,
    element::ElementInstance,
    global::GlobalInstance,
    memory::MemoryInstance,
    table::{TableElement, TableInstance},
};
use crate::types::{
    instructions::ConstInstruction, Addr, Data, DataAddr, DataKind, ElementItem, ElementKind, ExternVal, FuncAddr,
    FuncType, Global, GlobalAddr, ImportKind, MemAddr, MemoryArch, MemoryType, Module, TableAddr, TableType,
    WasmFunction,
};
use crate::{VecExt, CALL_STACK_SIZE};

/// An instantiated Wasm module on which function can be called
#[allow(dead_code)]
#[derive(Debug, Default)]
pub struct Instance {
    pub(crate) module: Module,

    pub(crate) funcs: Vec<Function>,
    pub(crate) tables: Vec<TableInstance>,
    pub(crate) memories: Vec<MemoryInstance>,
    pub(crate) globals: Vec<GlobalInstance>,
    pub(crate) elements: Vec<ElementInstance>,
    pub(crate) datas: Vec<DataInstance>,
}

impl Instance {
    /// Instantiate the module with the given imports
    fn instantiate_raw(module: Module, imports: Imports) -> Result<Self> {
        let mut instance = Instance { module, ..Default::default() };

        let mut addrs = instance.resolve_imports(imports)?;

        addrs.funcs.extend(instance.init_funcs(instance.module.funcs.clone().into())?);
        addrs.tables.extend(instance.init_tables(instance.module.table_types.clone().into())?);
        addrs.memories.extend(instance.init_memories(instance.module.memory_types.clone().into())?);

        let global_addrs =
            instance.init_globals(addrs.globals, instance.module.globals.clone().into(), &addrs.funcs)?;

        let elem_trapped = instance.init_elements(&addrs.tables, &addrs.funcs, &global_addrs)?;
        if let Some(trap) = elem_trapped {
            return Err(Error::Trap(trap));
        }

        let data_trapped = instance.init_datas(&addrs.memories, instance.module.data.clone().into())?;
        if let Some(trap) = data_trapped {
            return Err(Error::Trap(trap));
        }

        Ok(instance)
    }

    /// Instantiate the module with the given imports and maybe restore state to resume execution of a function
    pub fn instantiate(
        module: Module,
        imports: Imports,
        state: Option<&[u8]>,
    ) -> Result<(Self, Option<Stack>, Vec<u8>)> {
        let mut instance = Self::instantiate_raw(module, imports)?;

        match state {
            Some(state) => {
                let decoder = flate2::read::ZlibDecoder::new(std::io::Cursor::new(state));
                let mut state: DeserializationState = bincode::deserialize_from(decoder)?;
                state.stack.call_stack.0.reserve_exact(CALL_STACK_SIZE);

                instance.memories[0] = state.memory;
                instance.globals.iter_mut().zip(state.globals.iter()).for_each(|(g, v)| g.value = *v);

                Ok((instance, Some(state.stack), state.extra_data))
            }
            None => Ok((instance, None, Vec::new())),
        }
    }

    /// Get a export by name
    pub(crate) fn export_addr(&self, name: &str) -> Option<ExternVal> {
        let export = self.module.exports.iter().find(|e| e.name == name.into())?;

        Some(ExternVal::new(export.kind, export.index))
    }

    #[inline]
    pub(crate) fn func_ty(&self, addr: FuncAddr) -> &FuncType {
        self.module.func_types.get(addr as usize).expect("No func type for func, this is a bug")
    }

    /// Get an exported function by name
    pub fn exported_func_untyped(self, name: &str) -> Result<FuncHandle> {
        let export = self.export_addr(name).ok_or_else(|| Error::Other(format!("Export not found: {}", name)))?;
        let ExternVal::Func(func_addr) = export else {
            return Err(Error::Other(format!("Export is not a function: {}", name)));
        };

        let func_inst = self.get_func(func_addr)?;
        let ty = func_inst.ty();

        Ok(FuncHandle { addr: func_addr, name: Some(name.to_string()), ty: ty.clone(), instance: self })
    }

    /// Get a typed exported function by name
    pub fn exported_func<P, R>(self, name: &str) -> Result<FuncHandleTyped<P, R>>
    where
        P: IntoWasmValueTuple,
        R: FromWasmValueTuple,
    {
        let func = self.exported_func_untyped(name)?;
        Ok(FuncHandleTyped { func, _marker: core::marker::PhantomData })
    }

    /// Get an exported memory by name
    pub fn exported_memory<'i>(&'i self, name: &str) -> Result<MemoryRef<'i>> {
        let export = self.export_addr(name).ok_or_else(|| Error::Other(format!("Export not found: {}", name)))?;
        let ExternVal::Memory(mem_addr) = export else {
            return Err(Error::Other(format!("Export is not a memory: {}", name)));
        };

        self.memory(mem_addr)
    }

    /// Get an exported memory by name
    pub fn exported_memory_mut<'i>(&'i mut self, name: &str) -> Result<MemoryRefMut<'i>> {
        let export = self.export_addr(name).ok_or_else(|| Error::Other(format!("Export not found: {}", name)))?;
        let ExternVal::Memory(mem_addr) = export else {
            return Err(Error::Other(format!("Export is not a memory: {}", name)));
        };

        self.memory_mut(mem_addr)
    }

    /// Get a memory by address
    pub(crate) fn memory(&self, addr: MemAddr) -> Result<MemoryRef<'_>> {
        let mem = self.get_mem(addr)?;
        Ok(MemoryRef { instance: mem })
    }

    /// Get a memory by address (mutable)
    pub(crate) fn memory_mut(&mut self, addr: MemAddr) -> Result<MemoryRefMut<'_>> {
        let mem = self.get_mem_mut(addr)?;
        Ok(MemoryRefMut { instance: mem })
    }
}

impl Instance {
    #[cold]
    pub(crate) fn not_found_error(name: &str) -> Error {
        Error::Other(format!("{} not found", name))
    }

    /// Get the function at the actual index in the store
    #[inline]
    pub(crate) fn get_func(&self, addr: FuncAddr) -> Result<&Function> {
        self.funcs.get(addr as usize).ok_or_else(|| Self::not_found_error("function"))
    }

    /// Get the memory at the actual index in the store
    #[inline]
    pub(crate) fn get_mem(&self, addr: MemAddr) -> Result<&MemoryInstance> {
        self.memories.get(addr as usize).ok_or_else(|| Self::not_found_error("memory"))
    }

    /// Get the mut memory at the actual index in the store
    #[inline]
    pub(crate) fn get_mem_mut(&mut self, addr: MemAddr) -> Result<&mut MemoryInstance> {
        self.memories.get_mut(addr as usize).ok_or_else(|| Self::not_found_error("memory"))
    }

    /// Get the table at the actual index in the store
    #[inline]
    pub(crate) fn get_table(&self, addr: TableAddr) -> Result<&TableInstance> {
        self.tables.get(addr as usize).ok_or_else(|| Self::not_found_error("table"))
    }

    /// Get the table at the actual index in the store
    #[inline]
    pub(crate) fn get_table_mut(&mut self, addr: TableAddr) -> Result<&mut TableInstance> {
        self.tables.get_mut(addr as usize).ok_or_else(|| Self::not_found_error("table"))
    }

    /// Get the data at the actual index in the store
    #[inline]
    pub(crate) fn get_data_mut(&mut self, addr: DataAddr) -> Result<&mut DataInstance> {
        self.datas.get_mut(addr as usize).ok_or_else(|| Self::not_found_error("data"))
    }

    /// Get the global at the actual index in the store
    #[inline]
    pub fn get_global_val(&self, addr: MemAddr) -> Result<RawWasmValue> {
        self.globals.get(addr as usize).ok_or_else(|| Self::not_found_error("global")).map(|global| global.value)
    }

    /// Set the global at the actual index in the store
    #[inline]
    pub(crate) fn set_global_val(&mut self, addr: MemAddr, value: RawWasmValue) -> Result<()> {
        let global = self.globals.get_mut_or_instance(addr, "global")?;
        global.value = value;
        Ok(())
    }
}

impl Instance {
    pub(crate) fn resolve_imports(&mut self, mut imports: Imports) -> Result<ResolvedImports> {
        let mut addrs = ResolvedImports::new();

        for import in self.module.imports.iter() {
            let val = imports.take(import).ok_or_else(|| LinkingError::unknown_import(import))?;

            // A link to something that needs to be added to the store
            match (val, &import.kind) {
                (Extern::Global { ty, val }, ImportKind::Global(import_ty)) => {
                    Imports::compare_types(import, &ty, import_ty)?;
                    addrs.globals.push(self.globals.add(GlobalInstance::new(val.into())) as u32);
                }
                (Extern::Table { ty, .. }, ImportKind::Table(import_ty)) => {
                    Imports::compare_table_types(import, &ty, import_ty)?;
                    addrs.tables.push(self.tables.add(TableInstance::new(ty)) as u32);
                }
                (Extern::Memory { ty }, ImportKind::Memory(import_ty)) => {
                    Imports::compare_memory_types(import, &ty, import_ty, None)?;
                    if let MemoryArch::I64 = ty.arch {
                        return Err(Error::UnsupportedFeature("64-bit memories".to_string()));
                    }
                    addrs.memories.push(self.memories.add(MemoryInstance::new(ty)) as u32);
                }
                (Extern::Function(Some(extern_func)), ImportKind::Function(ty)) => {
                    let import_func_type = self
                        .module
                        .func_types
                        .get(*ty as usize)
                        .ok_or_else(|| LinkingError::incompatible_import_type(import))?;

                    Imports::compare_types(import, extern_func.ty(), import_func_type)?;
                    addrs.funcs.push(self.funcs.add(extern_func) as u32);
                }
                _ => return Err(LinkingError::incompatible_import_type(import).into()),
            }
        }

        Ok(addrs)
    }

    /// Add functions to the store, returning their addresses in the store
    pub(crate) fn init_funcs(&mut self, funcs: Vec<WasmFunction>) -> Result<Vec<FuncAddr>> {
        let func_count = self.funcs.len();
        let mut func_addrs = Vec::with_capacity(func_count);
        for (i, func) in funcs.into_iter().enumerate() {
            self.funcs.push(Function::Wasm(func));
            func_addrs.push((i + func_count) as FuncAddr);
        }
        Ok(func_addrs)
    }

    /// Add tables to the store, returning their addresses in the store
    pub(crate) fn init_tables(&mut self, tables: Vec<TableType>) -> Result<Vec<TableAddr>> {
        let table_count = self.tables.len();
        let mut table_addrs = Vec::with_capacity(table_count);
        for (i, table) in tables.into_iter().enumerate() {
            self.tables.push(TableInstance::new(table));
            table_addrs.push((i + table_count) as TableAddr);
        }
        Ok(table_addrs)
    }

    /// Add memories to the store, returning their addresses in the store
    pub(crate) fn init_memories(&mut self, memories: Vec<MemoryType>) -> Result<Vec<MemAddr>> {
        let mem_count = self.memories.len();
        let mut mem_addrs = Vec::with_capacity(mem_count);
        for (i, mem) in memories.into_iter().enumerate() {
            if let MemoryArch::I64 = mem.arch {
                return Err(Error::UnsupportedFeature("64-bit memories".to_string()));
            }
            self.memories.push(MemoryInstance::new(mem));
            mem_addrs.push((i + mem_count) as MemAddr);
        }
        Ok(mem_addrs)
    }

    /// Add globals to the store, returning their addresses in the store
    pub(crate) fn init_globals(
        &mut self,
        mut imported_globals: Vec<GlobalAddr>,
        new_globals: Vec<Global>,
        func_addrs: &[FuncAddr],
    ) -> Result<Vec<Addr>> {
        let global_count = self.globals.len();
        imported_globals.reserve_exact(new_globals.len());
        let mut global_addrs = imported_globals;

        for (i, global) in new_globals.iter().enumerate() {
            self.globals.push(GlobalInstance::new(self.eval_const(&global.init, &global_addrs, func_addrs)?));
            global_addrs.push((i + global_count) as Addr);
        }

        Ok(global_addrs)
    }

    fn elem_addr(&self, item: &ElementItem, globals: &[Addr], funcs: &[FuncAddr]) -> Result<Option<u32>> {
        let res = match item {
            ElementItem::Func(addr) | ElementItem::Expr(ConstInstruction::RefFunc(addr)) => {
                Some(funcs.get(*addr as usize).copied().ok_or_else(|| {
                    Error::Other(format!("function {} not found. This should have been caught by the validator", addr))
                })?)
            }
            ElementItem::Expr(ConstInstruction::RefNull(_ty)) => None,
            ElementItem::Expr(ConstInstruction::GlobalGet(addr)) => {
                let addr = globals.get(*addr as usize).copied().ok_or_else(|| {
                    Error::Other(format!("global {} not found. This should have been caught by the validator", addr))
                })?;
                let val: i64 = self.globals[addr as usize].value.into();

                // check if the global is actually a null reference
                match val < 0 {
                    true => None,
                    false => Some(val as u32),
                }
            }
            _ => return Err(Error::UnsupportedFeature(format!("const expression other than ref: {:?}", item))),
        };

        Ok(res)
    }

    /// Add elements to the store, returning their addresses in the store
    /// Should be called after the tables have been added
    pub(crate) fn init_elements(
        &mut self,
        table_addrs: &[TableAddr],
        func_addrs: &[FuncAddr],
        global_addrs: &[Addr],
    ) -> Result<Option<Trap>> {
        // let elem_count = self.elements.len();
        // let mut elem_addrs = Vec::with_capacity(elem_count);
        for (i, element) in self.module.elements.iter().enumerate() {
            let init = element
                .items
                .iter()
                .map(|item| Ok(TableElement::from(self.elem_addr(item, global_addrs, func_addrs)?)))
                .collect::<Result<Vec<_>>>()?;

            let items = match element.kind {
                // doesn't need to be initialized, can be initialized lazily using the `table.init` instruction
                ElementKind::Passive => Some(init),

                // this one is not available to the runtime but needs to be initialized to declare references
                ElementKind::Declared => None, // a. Execute the instruction elm.drop i

                // this one is active, so we need to initialize it (essentially a `table.init` instruction)
                ElementKind::Active { offset, table } => {
                    let offset = self.eval_i32_const(&offset)?;
                    let table_addr = table_addrs
                        .get(table as usize)
                        .copied()
                        .ok_or_else(|| Error::Other(format!("table {} not found for element {}", table, i)))?;

                    let Some(table) = self.tables.get_mut(table_addr as usize) else {
                        return Err(Error::Other(format!("table {} not found for element {}", table, i)));
                    };

                    if let Err(Error::Trap(trap)) = table.init_raw(offset, &init) {
                        return Ok(Some(trap));
                    }

                    None
                }
            };

            self.elements.push(ElementInstance::new(element.kind, items));
            // elem_addrs.push((i + elem_count) as Addr);
        }

        // this should be optimized out by the compiler
        Ok(None)
    }

    /// Add data to the store, returning their addresses in the store
    pub(crate) fn init_datas(&mut self, mem_addrs: &[MemAddr], datas: Vec<Data>) -> Result<Option<Trap>> {
        let data_count = self.datas.len();
        let mut data_addrs = Vec::with_capacity(data_count);
        for (i, data) in datas.into_iter().enumerate() {
            let data_val = match data.kind {
                DataKind::Active { mem: mem_addr, offset } => {
                    // a. Assert: memidx == 0
                    if mem_addr != 0 {
                        return Err(Error::UnsupportedFeature("data segments for non-zero memories".to_string()));
                    }

                    let Some(mem_addr) = mem_addrs.get(mem_addr as usize) else {
                        return Err(Error::Other(format!("memory {} not found for data segment {}", mem_addr, i)));
                    };

                    let offset = self.eval_i32_const(&offset)?;
                    let Some(mem) = self.memories.get_mut(*mem_addr as usize) else {
                        return Err(Error::Other(format!("memory {} not found for data segment {}", mem_addr, i)));
                    };

                    match mem.store(offset as usize, data.data.len(), &data.data) {
                        Ok(()) => None,
                        Err(Error::Trap(trap)) => return Ok(Some(trap)),
                        Err(e) => return Err(e),
                    }
                }
                DataKind::Passive => Some(data.data.to_vec()),
            };

            self.datas.push(DataInstance::new(data_val));
            data_addrs.push((i + data_count) as Addr);
        }

        // this should be optimized out by the compiler
        Ok(None)
    }

    /// Evaluate a constant expression, only supporting i32 globals and i32.const
    pub(crate) fn eval_i32_const(&self, const_instr: &ConstInstruction) -> Result<i32> {
        use ConstInstruction::*;
        let val = match const_instr {
            I32Const(i) => *i,
            GlobalGet(addr) => i32::from(self.globals[*addr as usize].value),
            _ => return Err(Error::Other("expected i32".to_string())),
        };
        Ok(val)
    }

    /// Evaluate a constant expression
    pub(crate) fn eval_const(
        &self,
        const_instr: &ConstInstruction,
        module_global_addrs: &[Addr],
        module_func_addrs: &[FuncAddr],
    ) -> Result<RawWasmValue> {
        use ConstInstruction::*;
        let val = match const_instr {
            F32Const(f) => RawWasmValue::from(*f),
            F64Const(f) => RawWasmValue::from(*f),
            I32Const(i) => RawWasmValue::from(*i),
            I64Const(i) => RawWasmValue::from(*i),
            GlobalGet(addr) => {
                let addr = module_global_addrs.get(*addr as usize).ok_or_else(|| {
                    Error::Other(format!("global {} not found. This should have been caught by the validator", addr))
                })?;

                let global = self.globals.get(*addr as usize).expect("global not found. This should be unreachable");
                global.value
            }
            RefNull(t) => RawWasmValue::from(t.default_value()),
            RefFunc(idx) => RawWasmValue::from(*module_func_addrs.get(*idx as usize).ok_or_else(|| {
                Error::Other(format!("function {} not found. This should have been caught by the validator", idx))
            })?),
        };
        Ok(val)
    }
}
