use alloc::format;
use alloc::string::ToString;
use core::ops::{BitAnd, BitOr, BitXor, Neg};

use crate::error::{Error, Result, Trap};
use crate::imports::{FuncContext, Function};
use crate::instance::Instance;
use crate::runtime::{BlockFrame, BlockType, CallFrame, RawWasmValue, Stack};
use crate::types::{instructions::BlockArgs, value::ValType, ElementKind};
use crate::{cold, unlikely, VecExt};

mod macros;
mod traits;
use {macros::*, traits::*};

/// The Wasm interpreter.
#[derive(Debug, Default)]
pub(crate) struct Interpreter {}

impl Interpreter {
    pub(crate) fn exec(&self, mut instance: &mut Instance, stack: &mut Stack, max_cycles: usize) -> Result<bool> {
        let mut cf = stack.call_stack.pop()?;

        for _ in 0..=max_cycles {
            use crate::types::instructions::Instruction::*;

            let curr_instr = cf.fetch_instr(&instance.funcs);

            match curr_instr {
                Nop => cold(),
                Unreachable => self.exec_unreachable()?,
                Drop => stack.values.pop().map(|_| ())?,
                Select(_valtype) => self.exec_select(stack)?,

                Call(v) => skip!(self.exec_call(v, stack, &mut cf, instance)),
                CallIndirect(ty, table) => {
                    skip!(self.exec_call_indirect(ty, table, stack, &mut cf, instance))
                }
                If(args, el, end) => {
                    skip!(self.exec_if((args).into(), el, end, stack, &mut cf, instance))
                }
                Loop(args, end) => self.enter_block(stack, cf.instr_ptr, end, BlockType::Loop, args, instance),
                Block(args, end) => self.enter_block(stack, cf.instr_ptr, end, BlockType::Block, args, instance),

                Br(v) => break_to!(cf, stack, module, store, v),
                BrIf(v) => {
                    if i32::from(stack.values.pop()?) != 0 {
                        break_to!(cf, stack, module, store, v);
                    }
                }
                BrTable(default, len) => {
                    let start = cf.instr_ptr + 1;
                    let end = start + len as usize;
                    if end > cf.instructions(&instance.funcs).len() {
                        return Err(Error::Other(format!(
                            "br_table out of bounds: {} >= {}",
                            end,
                            cf.instructions(&instance.funcs).len()
                        )));
                    }

                    let idx: i32 = stack.values.pop()?.into();
                    match cf.instructions(&instance.funcs)[start..end].get(idx as usize) {
                        None => break_to!(cf, stack, module, store, default),
                        Some(BrLabel(to)) => break_to!(cf, stack, module, store, *to),
                        _ => return Err(Error::Other("br_table with invalid label".to_string())),
                    }
                }

                Return => match stack.call_stack.is_empty() {
                    true => return Ok(true),
                    false => call!(cf, stack, module, store),
                },

                // We're essentially using else as a EndBlockFrame instruction for if blocks
                Else(end_offset) => self.exec_else(stack, end_offset, &mut cf)?,

                // remove the label from the label stack
                EndBlockFrame => self.exec_end_block(stack)?,

                LocalGet(local_index) => self.exec_local_get(local_index, stack, &cf),
                LocalSet(local_index) => self.exec_local_set(local_index, stack, &mut cf)?,
                LocalTee(local_index) => self.exec_local_tee(local_index, stack, &mut cf)?,

                GlobalGet(global_index) => self.exec_global_get(global_index, stack, instance)?,
                GlobalSet(global_index) => self.exec_global_set(global_index, stack, instance)?,

                I32Const(val) => self.exec_const(val, stack),
                I64Const(val) => self.exec_const(val, stack),
                F32Const(val) => self.exec_const(val, stack),
                F64Const(val) => self.exec_const(val, stack),

                MemorySize(addr, byte) => self.exec_memory_size(addr, byte, stack, instance)?,
                MemoryGrow(addr, byte) => self.exec_memory_grow(addr, byte, stack, instance)?,

                // Bulk memory operations
                MemoryCopy(from, to) => self.exec_memory_copy(from, to, stack, instance)?,
                MemoryFill(addr) => self.exec_memory_fill(addr, stack, instance)?,
                MemoryInit(data_idx, mem_idx) => self.exec_memory_init(data_idx, mem_idx, stack, instance)?,
                DataDrop(data_index) => instance.get_data_mut(data_index)?.drop(),

                I32Store { mem_addr, offset } => {
                    mem_store!(i32, (mem_addr, offset), stack, instance)
                }
                I64Store { mem_addr, offset } => {
                    mem_store!(i64, (mem_addr, offset), stack, instance)
                }
                F32Store { mem_addr, offset } => {
                    mem_store!(f32, (mem_addr, offset), stack, instance)
                }
                F64Store { mem_addr, offset } => {
                    mem_store!(f64, (mem_addr, offset), stack, instance)
                }
                I32Store8 { mem_addr, offset } => {
                    mem_store!(i8, i32, (mem_addr, offset), stack, instance)
                }
                I32Store16 { mem_addr, offset } => {
                    mem_store!(i16, i32, (mem_addr, offset), stack, instance)
                }
                I64Store8 { mem_addr, offset } => {
                    mem_store!(i8, i64, (mem_addr, offset), stack, instance)
                }
                I64Store16 { mem_addr, offset } => {
                    mem_store!(i16, i64, (mem_addr, offset), stack, instance)
                }
                I64Store32 { mem_addr, offset } => {
                    mem_store!(i32, i64, (mem_addr, offset), stack, instance)
                }

                I32Load { mem_addr, offset } => mem_load!(i32, (mem_addr, offset), stack, instance),
                I64Load { mem_addr, offset } => mem_load!(i64, (mem_addr, offset), stack, instance),
                F32Load { mem_addr, offset } => mem_load!(f32, (mem_addr, offset), stack, instance),
                F64Load { mem_addr, offset } => mem_load!(f64, (mem_addr, offset), stack, instance),
                I32Load8S { mem_addr, offset } => {
                    mem_load!(i8, i32, (mem_addr, offset), stack, instance)
                }
                I32Load8U { mem_addr, offset } => {
                    mem_load!(u8, i32, (mem_addr, offset), stack, instance)
                }
                I32Load16S { mem_addr, offset } => {
                    mem_load!(i16, i32, (mem_addr, offset), stack, instance)
                }
                I32Load16U { mem_addr, offset } => {
                    mem_load!(u16, i32, (mem_addr, offset), stack, instance)
                }
                I64Load8S { mem_addr, offset } => {
                    mem_load!(i8, i64, (mem_addr, offset), stack, instance)
                }
                I64Load8U { mem_addr, offset } => {
                    mem_load!(u8, i64, (mem_addr, offset), stack, instance)
                }
                I64Load16S { mem_addr, offset } => {
                    mem_load!(i16, i64, (mem_addr, offset), stack, instance)
                }
                I64Load16U { mem_addr, offset } => {
                    mem_load!(u16, i64, (mem_addr, offset), stack, instance)
                }
                I64Load32S { mem_addr, offset } => {
                    mem_load!(i32, i64, (mem_addr, offset), stack, instance)
                }
                I64Load32U { mem_addr, offset } => {
                    mem_load!(u32, i64, (mem_addr, offset), stack, instance)
                }

                I64Eqz => comp_zero!(==, i64, stack),
                I32Eqz => comp_zero!(==, i32, stack),

                I32Eq => comp!(==, i32, stack),
                I64Eq => comp!(==, i64, stack),
                F32Eq => comp!(==, f32, stack),
                F64Eq => comp!(==, f64, stack),

                I32Ne => comp!(!=, i32, stack),
                I64Ne => comp!(!=, i64, stack),
                F32Ne => comp!(!=, f32, stack),
                F64Ne => comp!(!=, f64, stack),

                I32LtS => comp!(<, i32, stack),
                I64LtS => comp!(<, i64, stack),
                I32LtU => comp!(<, u32, stack),
                I64LtU => comp!(<, u64, stack),
                F32Lt => comp!(<, f32, stack),
                F64Lt => comp!(<, f64, stack),

                I32LeS => comp!(<=, i32, stack),
                I64LeS => comp!(<=, i64, stack),
                I32LeU => comp!(<=, u32, stack),
                I64LeU => comp!(<=, u64, stack),
                F32Le => comp!(<=, f32, stack),
                F64Le => comp!(<=, f64, stack),

                I32GeS => comp!(>=, i32, stack),
                I64GeS => comp!(>=, i64, stack),
                I32GeU => comp!(>=, u32, stack),
                I64GeU => comp!(>=, u64, stack),
                F32Ge => comp!(>=, f32, stack),
                F64Ge => comp!(>=, f64, stack),

                I32GtS => comp!(>, i32, stack),
                I64GtS => comp!(>, i64, stack),
                I32GtU => comp!(>, u32, stack),
                I64GtU => comp!(>, u64, stack),
                F32Gt => comp!(>, f32, stack),
                F64Gt => comp!(>, f64, stack),

                I64Add => arithmetic!(wrapping_add, i64, stack),
                I32Add => arithmetic!(wrapping_add, i32, stack),
                F32Add => arithmetic!(+, f32, stack),
                F64Add => arithmetic!(+, f64, stack),

                I32Sub => arithmetic!(wrapping_sub, i32, stack),
                I64Sub => arithmetic!(wrapping_sub, i64, stack),
                F32Sub => arithmetic!(-, f32, stack),
                F64Sub => arithmetic!(-, f64, stack),

                F32Div => arithmetic!(/, f32, stack),
                F64Div => arithmetic!(/, f64, stack),

                I32Mul => arithmetic!(wrapping_mul, i32, stack),
                I64Mul => arithmetic!(wrapping_mul, i64, stack),
                F32Mul => arithmetic!(*, f32, stack),
                F64Mul => arithmetic!(*, f64, stack),

                // these can trap
                I32DivS => checked_int_arithmetic!(checked_div, i32, stack),
                I64DivS => checked_int_arithmetic!(checked_div, i64, stack),
                I32DivU => checked_int_arithmetic!(checked_div, u32, stack),
                I64DivU => checked_int_arithmetic!(checked_div, u64, stack),

                I32RemS => checked_int_arithmetic!(checked_wrapping_rem, i32, stack),
                I64RemS => checked_int_arithmetic!(checked_wrapping_rem, i64, stack),
                I32RemU => checked_int_arithmetic!(checked_wrapping_rem, u32, stack),
                I64RemU => checked_int_arithmetic!(checked_wrapping_rem, u64, stack),

                I32And => arithmetic!(bitand, i32, stack),
                I64And => arithmetic!(bitand, i64, stack),
                I32Or => arithmetic!(bitor, i32, stack),
                I64Or => arithmetic!(bitor, i64, stack),
                I32Xor => arithmetic!(bitxor, i32, stack),
                I64Xor => arithmetic!(bitxor, i64, stack),
                I32Shl => arithmetic!(wasm_shl, i32, stack),
                I64Shl => arithmetic!(wasm_shl, i64, stack),
                I32ShrS => arithmetic!(wasm_shr, i32, stack),
                I64ShrS => arithmetic!(wasm_shr, i64, stack),
                I32ShrU => arithmetic!(wasm_shr, u32, stack),
                I64ShrU => arithmetic!(wasm_shr, u64, stack),
                I32Rotl => arithmetic!(wasm_rotl, i32, stack),
                I64Rotl => arithmetic!(wasm_rotl, i64, stack),
                I32Rotr => arithmetic!(wasm_rotr, i32, stack),
                I64Rotr => arithmetic!(wasm_rotr, i64, stack),

                I32Clz => arithmetic_single!(leading_zeros, i32, stack),
                I64Clz => arithmetic_single!(leading_zeros, i64, stack),
                I32Ctz => arithmetic_single!(trailing_zeros, i32, stack),
                I64Ctz => arithmetic_single!(trailing_zeros, i64, stack),
                I32Popcnt => arithmetic_single!(count_ones, i32, stack),
                I64Popcnt => arithmetic_single!(count_ones, i64, stack),

                F32ConvertI32S => conv!(i32, f32, stack),
                F32ConvertI64S => conv!(i64, f32, stack),
                F64ConvertI32S => conv!(i32, f64, stack),
                F64ConvertI64S => conv!(i64, f64, stack),
                F32ConvertI32U => conv!(u32, f32, stack),
                F32ConvertI64U => conv!(u64, f32, stack),
                F64ConvertI32U => conv!(u32, f64, stack),
                F64ConvertI64U => conv!(u64, f64, stack),
                I32Extend8S => conv!(i8, i32, stack),
                I32Extend16S => conv!(i16, i32, stack),
                I64Extend8S => conv!(i8, i64, stack),
                I64Extend16S => conv!(i16, i64, stack),
                I64Extend32S => conv!(i32, i64, stack),
                I64ExtendI32U => conv!(u32, i64, stack),
                I64ExtendI32S => conv!(i32, i64, stack),
                I32WrapI64 => conv!(i64, i32, stack),

                F32DemoteF64 => conv!(f64, f32, stack),
                F64PromoteF32 => conv!(f32, f64, stack),

                F32Abs => arithmetic_single!(abs, f32, stack),
                F64Abs => arithmetic_single!(abs, f64, stack),
                F32Neg => arithmetic_single!(neg, f32, stack),
                F64Neg => arithmetic_single!(neg, f64, stack),
                F32Ceil => arithmetic_single!(ceil, f32, stack),
                F64Ceil => arithmetic_single!(ceil, f64, stack),
                F32Floor => arithmetic_single!(floor, f32, stack),
                F64Floor => arithmetic_single!(floor, f64, stack),
                F32Trunc => arithmetic_single!(trunc, f32, stack),
                F64Trunc => arithmetic_single!(trunc, f64, stack),
                F32Nearest => arithmetic_single!(tw_nearest, f32, stack),
                F64Nearest => arithmetic_single!(tw_nearest, f64, stack),
                F32Sqrt => arithmetic_single!(sqrt, f32, stack),
                F64Sqrt => arithmetic_single!(sqrt, f64, stack),
                F32Min => arithmetic!(tw_minimum, f32, stack),
                F64Min => arithmetic!(tw_minimum, f64, stack),
                F32Max => arithmetic!(tw_maximum, f32, stack),
                F64Max => arithmetic!(tw_maximum, f64, stack),
                F32Copysign => arithmetic!(copysign, f32, stack),
                F64Copysign => arithmetic!(copysign, f64, stack),

                // no-op instructions since types are erased at runtime
                I32ReinterpretF32 | I64ReinterpretF64 | F32ReinterpretI32 | F64ReinterpretI64 => {}

                // unsigned versions of these are a bit broken atm
                I32TruncF32S => checked_conv_float!(f32, i32, stack),
                I32TruncF64S => checked_conv_float!(f64, i32, stack),
                I32TruncF32U => checked_conv_float!(f32, u32, i32, stack),
                I32TruncF64U => checked_conv_float!(f64, u32, i32, stack),
                I64TruncF32S => checked_conv_float!(f32, i64, stack),
                I64TruncF64S => checked_conv_float!(f64, i64, stack),
                I64TruncF32U => checked_conv_float!(f32, u64, i64, stack),
                I64TruncF64U => checked_conv_float!(f64, u64, i64, stack),

                TableGet(table_idx) => self.exec_table_get(table_idx, stack, instance)?,
                TableSet(table_idx) => self.exec_table_set(table_idx, stack, instance)?,
                TableSize(table_idx) => self.exec_table_size(table_idx, stack, instance)?,
                TableInit(table_idx, elem_idx) => self.exec_table_init(elem_idx, table_idx, instance)?,

                I32TruncSatF32S => arithmetic_single!(trunc, f32, i32, stack),
                I32TruncSatF32U => arithmetic_single!(trunc, f32, u32, stack),
                I32TruncSatF64S => arithmetic_single!(trunc, f64, i32, stack),
                I32TruncSatF64U => arithmetic_single!(trunc, f64, u32, stack),
                I64TruncSatF32S => arithmetic_single!(trunc, f32, i64, stack),
                I64TruncSatF32U => arithmetic_single!(trunc, f32, u64, stack),
                I64TruncSatF64S => arithmetic_single!(trunc, f64, i64, stack),
                I64TruncSatF64U => arithmetic_single!(trunc, f64, u64, stack),

                // custom instructions
                LocalGet2(a, b) => self.exec_local_get2(a, b, stack, &cf),
                LocalGet3(a, b, c) => self.exec_local_get3(a, b, c, stack, &cf),
                LocalTeeGet(a, b) => self.exec_local_tee_get(a, b, stack, &mut cf),
                LocalGetSet(a, b) => self.exec_local_get_set(a, b, &mut cf),
                I64XorConstRotl(rotate_by) => self.exec_i64_xor_const_rotl(rotate_by, stack)?,
                I32LocalGetConstAdd(local, val) => self.exec_i32_local_get_const_add(local, val, stack, &cf),
                I32StoreLocal { local, const_i32: consti32, offset, mem_addr } => {
                    self.exec_i32_store_local(local, consti32, offset, mem_addr, &cf, instance)?
                }
                i => {
                    cold();
                    return Err(Error::UnsupportedFeature(format!("unimplemented instruction: {:?}", i)));
                }
            };

            cf.instr_ptr += 1;
        }

        stack.call_stack.push(cf)?;

        Ok(false)
    }

    #[inline(always)]
    fn exec_end_block(&self, stack: &mut Stack) -> Result<()> {
        let block = stack.blocks.pop()?;
        stack.values.truncate_keep(block.stack_ptr, block.results as u32);
        Ok(())
    }

    #[inline(always)]
    fn exec_else(&self, stack: &mut Stack, end_offset: u32, cf: &mut CallFrame) -> Result<()> {
        let block = stack.blocks.pop()?;
        stack.values.truncate_keep(block.stack_ptr, block.results as u32);
        cf.instr_ptr += end_offset as usize;
        Ok(())
    }

    #[inline(always)]
    #[cold]
    fn exec_unreachable(&self) -> Result<()> {
        Err(Error::Trap(Trap::Unreachable))
    }

    #[inline(always)]
    fn exec_const(&self, val: impl Into<RawWasmValue>, stack: &mut Stack) {
        stack.values.push(val.into());
    }

    #[allow(clippy::too_many_arguments)]
    #[inline(always)]
    fn exec_i32_store_local(
        &self,
        local: u32,
        const_i32: i32,
        offset: u32,
        mem_addr: u8,
        cf: &CallFrame,
        instance: &mut Instance,
    ) -> Result<()> {
        let mem = instance.get_mem_mut(mem_addr as u32)?;
        let val = const_i32.to_le_bytes();
        let addr: u64 = cf.get_local(local).into();
        mem.store((offset as u64 + addr) as usize, val.len(), &val)?;
        Ok(())
    }

    #[inline(always)]
    fn exec_i32_local_get_const_add(&self, local: u32, val: i32, stack: &mut Stack, cf: &CallFrame) {
        let local: i32 = cf.get_local(local).into();
        stack.values.push((local + val).into());
    }

    #[inline(always)]
    fn exec_i64_xor_const_rotl(&self, rotate_by: i64, stack: &mut Stack) -> Result<()> {
        let val: i64 = stack.values.pop()?.into();
        let res = stack.values.last_mut()?;
        let mask: i64 = (*res).into();
        *res = (val ^ mask).rotate_left(rotate_by as u32).into();
        Ok(())
    }

    #[inline(always)]
    fn exec_local_get(&self, local_index: u32, stack: &mut Stack, cf: &CallFrame) {
        stack.values.push(cf.get_local(local_index));
    }

    #[inline(always)]
    fn exec_local_get2(&self, a: u32, b: u32, stack: &mut Stack, cf: &CallFrame) {
        stack.values.push(cf.get_local(a));
        stack.values.push(cf.get_local(b));
    }

    #[inline(always)]
    fn exec_local_get3(&self, a: u32, b: u32, c: u32, stack: &mut Stack, cf: &CallFrame) {
        stack.values.push(cf.get_local(a));
        stack.values.push(cf.get_local(b));
        stack.values.push(cf.get_local(c));
    }

    #[inline(always)]
    fn exec_local_get_set(&self, a: u32, b: u32, cf: &mut CallFrame) {
        cf.set_local(b, cf.get_local(a))
    }

    #[inline(always)]
    fn exec_local_set(&self, local_index: u32, stack: &mut Stack, cf: &mut CallFrame) -> Result<()> {
        cf.set_local(local_index, stack.values.pop()?);
        Ok(())
    }

    #[inline(always)]
    fn exec_local_tee(&self, local_index: u32, stack: &mut Stack, cf: &mut CallFrame) -> Result<()> {
        cf.set_local(local_index, *stack.values.last()?);
        Ok(())
    }

    #[inline(always)]
    fn exec_local_tee_get(&self, a: u32, b: u32, stack: &mut Stack, cf: &mut CallFrame) {
        let last =
            stack.values.last().expect("localtee: stack is empty. this should have been validated by the parser");
        cf.set_local(a, *last);
        stack.values.push(match a == b {
            true => *last,
            false => cf.get_local(b),
        });
    }

    #[inline(always)]
    fn exec_global_get(&self, global_index: u32, stack: &mut Stack, module: &Instance) -> Result<()> {
        let global = module.get_global_val(global_index)?;
        stack.values.push(global);
        Ok(())
    }

    #[inline(always)]
    fn exec_global_set(&self, global_index: u32, stack: &mut Stack, instance: &mut Instance) -> Result<()> {
        instance.set_global_val(global_index, stack.values.pop()?)?;
        Ok(())
    }

    #[inline(always)]
    fn exec_table_get(&self, table_index: u32, stack: &mut Stack, instance: &Instance) -> Result<()> {
        let table = instance.get_table(table_index)?;
        let idx: u32 = stack.values.pop()?.into();
        let v = table.get_wasm_val(idx)?;
        stack.values.push(v.into());
        Ok(())
    }

    #[inline(always)]
    fn exec_table_set(&self, table_index: u32, stack: &mut Stack, instance: &mut Instance) -> Result<()> {
        let table = instance.get_table_mut(table_index)?;
        let val = stack.values.pop()?.into();
        let idx = stack.values.pop()?.into();
        table.set(idx, val)?;
        Ok(())
    }

    #[inline(always)]
    fn exec_table_size(&self, table_index: u32, stack: &mut Stack, module: &Instance) -> Result<()> {
        let table = module.get_table(table_index)?;
        stack.values.push(table.size().into());
        Ok(())
    }

    #[inline(always)]
    fn exec_table_init(&self, elem_index: u32, table_index: u32, instance: &mut Instance) -> Result<()> {
        let table = instance.tables.get_mut_or_instance(table_index, "table")?;
        let elem = instance.elements.get_or_instance(elem_index, "element")?;

        if let ElementKind::Passive = elem.kind {
            return Err(Trap::TableOutOfBounds { offset: 0, len: 0, max: 0 }.into());
        }

        let Some(items) = elem.items.as_ref() else {
            return Err(Trap::TableOutOfBounds { offset: 0, len: 0, max: 0 }.into());
        };

        table.init(0, items)?;
        Ok(())
    }

    #[inline(always)]
    fn exec_select(&self, stack: &mut Stack) -> Result<()> {
        let cond: i32 = stack.values.pop()?.into();
        let val2 = stack.values.pop()?;
        // if cond != 0, we already have the right value on the stack
        if cond == 0 {
            *stack.values.last_mut()? = val2;
        }
        Ok(())
    }

    #[inline(always)]
    fn exec_memory_size(&self, addr: u32, byte: u8, stack: &mut Stack, module: &Instance) -> Result<()> {
        if unlikely(byte != 0) {
            return Err(Error::UnsupportedFeature("memory.size with byte != 0".to_string()));
        }

        let mem = module.get_mem(addr)?;
        stack.values.push((mem.page_count() as i32).into());
        Ok(())
    }

    #[inline(always)]
    fn exec_memory_grow(&self, addr: u32, byte: u8, stack: &mut Stack, instance: &mut Instance) -> Result<()> {
        if unlikely(byte != 0) {
            return Err(Error::UnsupportedFeature("memory.grow with byte != 0".to_string()));
        }

        let mem = instance.get_mem_mut(addr)?;
        let prev_size = mem.page_count() as i32;
        let pages_delta = stack.values.last_mut()?;
        *pages_delta = match mem.grow(i32::from(*pages_delta)) {
            Some(_) => prev_size.into(),
            None => (-1).into(),
        };

        Ok(())
    }

    #[inline(always)]
    fn exec_memory_copy(&self, from: u32, to: u32, stack: &mut Stack, instance: &mut Instance) -> Result<()> {
        let size: i32 = stack.values.pop()?.into();
        let src: i32 = stack.values.pop()?.into();
        let dst: i32 = stack.values.pop()?.into();

        if from == to {
            let mem_from = instance.get_mem_mut(from)?;
            // copy within the same memory
            mem_from.copy_within(dst as usize, src as usize, size as usize)?;
        } else {
            // copy between two memories
            todo!("Copy between different memories not supported");
            // let mem_from = instance.get_mem(from)?;
            // let mut mem_to = instance.get_mem_mut(to)?;
            // mem_to.copy_from_slice(dst as usize, mem_from.load(src as usize, size as usize)?)?;
        }
        Ok(())
    }

    #[inline(always)]
    fn exec_memory_fill(&self, addr: u32, stack: &mut Stack, instance: &mut Instance) -> Result<()> {
        let size: i32 = stack.values.pop()?.into();
        let val: i32 = stack.values.pop()?.into();
        let dst: i32 = stack.values.pop()?.into();

        let mem = instance.get_mem_mut(addr)?;
        mem.fill(dst as usize, size as usize, val as u8)?;
        Ok(())
    }

    #[inline(always)]
    fn exec_memory_init(
        &self,
        data_index: u32,
        mem_index: u32,
        stack: &mut Stack,
        instance: &mut Instance,
    ) -> Result<()> {
        let size = i32::from(stack.values.pop()?) as usize;
        let offset = i32::from(stack.values.pop()?) as usize;
        let dst = i32::from(stack.values.pop()?) as usize;

        let data = match &instance.datas.get(data_index as usize).ok_or_else(|| Instance::not_found_error("data"))?.data
        {
            Some(data) => data,
            None => return Err(Trap::MemoryOutOfBounds { offset: 0, len: 0, max: 0 }.into()),
        };

        if unlikely(offset + size > data.len()) {
            return Err(Trap::MemoryOutOfBounds { offset, len: size, max: data.len() }.into());
        }

        let mem = instance.memories.get_mut(mem_index as usize).ok_or_else(|| Instance::not_found_error("memory"))?;
        mem.store(dst, size, &data[offset..(offset + size)])?;
        Ok(())
    }

    #[inline(always)]
    fn exec_call(&self, v: u32, stack: &mut Stack, cf: &mut CallFrame, instance: &mut Instance) -> Result<()> {
        let func_inst = instance.funcs.get_or_instance(v, "function")?;
        let wasm_func = match &func_inst {
            Function::Wasm(wasm_func) => wasm_func,
            Function::Host(host_func) => {
                let params = stack.values.pop_params(&host_func.ty.params)?;
                let res = (host_func.func)(
                    FuncContext { module: &instance.module, memories: &mut instance.memories },
                    &params,
                );

                let res = match res {
                    Ok(res) => {
                        stack.values.extend_from_typed(&res);
                        Ok(())
                    }
                    Err(Error::PauseExecution) => Err(Error::PauseExecution),
                    Err(err) => return Err(err),
                };

                cf.instr_ptr += 1;
                return res;
            }
        };

        let params = stack.values.pop_n_rev(wasm_func.ty.params.len())?;
        let new_call_frame = CallFrame::new(v, wasm_func, params, stack.blocks.len() as u32);

        cf.instr_ptr += 1; // skip the call instruction
        stack.call_stack.push(core::mem::replace(cf, new_call_frame))?;
        Ok(())
    }

    #[inline(always)]
    fn exec_call_indirect(
        &self,
        type_addr: u32,
        table_addr: u32,
        stack: &mut Stack,
        cf: &mut CallFrame,
        instance: &mut Instance,
    ) -> Result<()> {
        let table = instance.tables.get_or_instance(table_addr, "table")?;
        let table_idx: u32 = stack.values.pop()?.into();

        // verify that the table is of the right type, this should be validated by the parser already
        let func_ref = {
            assert!(table.kind.element_type == ValType::RefFunc, "table is not of type funcref");
            table.get(table_idx)?.addr().ok_or(Trap::UninitializedElement { index: table_idx as usize })?
        };

        let func_inst = instance.funcs.get_or_instance(func_ref, "function")?;
        let call_ty = instance.func_ty(type_addr);

        let wasm_func = match &func_inst {
            Function::Wasm(ref f) => f,
            Function::Host(host_func) => {
                if unlikely(host_func.ty != *call_ty) {
                    return Err(Trap::IndirectCallTypeMismatch {
                        actual: host_func.ty.clone(),
                        expected: call_ty.clone(),
                    }
                    .into());
                }

                let params = stack.values.pop_params(&host_func.ty.params)?;
                let res = (host_func.func)(
                    FuncContext { module: &instance.module, memories: &mut instance.memories },
                    &params,
                );

                let res = match res {
                    Ok(res) => {
                        stack.values.extend_from_typed(&res);
                        Ok(())
                    }
                    Err(Error::PauseExecution) => Err(Error::PauseExecution),
                    Err(err) => return Err(err),
                };

                cf.instr_ptr += 1;
                return res;
            }
        };

        if unlikely(wasm_func.ty != *call_ty) {
            return Err(
                Trap::IndirectCallTypeMismatch { actual: wasm_func.ty.clone(), expected: call_ty.clone() }.into()
            );
        }

        let params = stack.values.pop_n_rev(wasm_func.ty.params.len())?;
        let new_call_frame = CallFrame::new(func_ref, wasm_func, params, stack.blocks.len() as u32);

        cf.instr_ptr += 1; // skip the call instruction
        stack.call_stack.push(core::mem::replace(cf, new_call_frame))?;

        Ok(())
    }

    #[inline(always)]
    fn exec_if(
        &self,
        args: BlockArgs,
        else_offset: u32,
        end_offset: u32,
        stack: &mut Stack,
        cf: &mut CallFrame,
        instance: &mut Instance,
    ) -> Result<()> {
        // truthy value is on the top of the stack, so enter the then block
        if i32::from(stack.values.pop()?) != 0 {
            self.enter_block(stack, cf.instr_ptr, end_offset, BlockType::If, args, instance);
            cf.instr_ptr += 1;
            return Ok(());
        }

        // falsy value is on the top of the stack
        if else_offset == 0 {
            cf.instr_ptr += end_offset as usize + 1;
            return Ok(());
        }

        let old = cf.instr_ptr;
        cf.instr_ptr += else_offset as usize;

        self.enter_block(stack, old + else_offset as usize, end_offset - else_offset, BlockType::Else, args, instance);

        cf.instr_ptr += 1;
        Ok(())
    }

    #[inline(always)]
    fn enter_block(
        &self,
        stack: &mut super::Stack,
        instr_ptr: usize,
        end_instr_offset: u32,
        ty: BlockType,
        args: BlockArgs,
        module: &Instance,
    ) {
        let (params, results) = match args {
            BlockArgs::Empty => (0, 0),
            BlockArgs::Type(_) => (0, 1),
            BlockArgs::FuncType(t) => {
                let ty = module.func_ty(t);
                (ty.params.len() as u8, ty.results.len() as u8)
            }
        };

        stack.blocks.push(BlockFrame {
            instr_ptr,
            end_instr_offset,
            stack_ptr: stack.values.len() as u32 - params as u32,
            results,
            params,
            ty,
        });
    }
}
