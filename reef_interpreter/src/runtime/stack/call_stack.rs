use alloc::{boxed::Box, vec::Vec};
use core::hint::unreachable_unchecked;

use crate::error::{Error, Result, Trap};
use crate::imports::Function;
use crate::runtime::{BlockType, RawWasmValue};
use crate::types::{instructions::Instruction, FuncAddr, LocalAddr, WasmFunction};
use crate::{cold, unlikely, CALL_STACK_SIZE};

#[derive(Debug, Clone, PartialEq, Eq, Default, serde::Serialize, serde::Deserialize)]
pub(crate) struct CallStack(pub(crate) Vec<CallFrame>);

impl CallStack {
    #[inline]
    pub(crate) fn new(initial_frame: CallFrame) -> Self {
        let mut stack = Vec::new();
        stack.reserve_exact(CALL_STACK_SIZE);
        stack.push(initial_frame);
        Self(stack)
    }

    #[inline]
    pub(crate) fn is_empty(&self) -> bool {
        self.0.is_empty()
    }

    #[inline(always)]
    pub(crate) fn pop(&mut self) -> Result<CallFrame> {
        match self.0.pop() {
            Some(frame) => Ok(frame),
            None => {
                cold();
                Err(Error::CallStackUnderflow)
            }
        }
    }

    #[inline(always)]
    pub(crate) fn push(&mut self, call_frame: CallFrame) -> Result<()> {
        if unlikely(self.0.len() >= self.0.capacity()) {
            return Err(Trap::CallStackOverflow.into());
        }
        self.0.push(call_frame);
        Ok(())
    }
}

#[derive(Debug, Clone, PartialEq, Eq, serde::Serialize, serde::Deserialize)]

pub(crate) struct CallFrame {
    pub(crate) instr_ptr: usize,
    pub(crate) block_ptr: u32,
    pub(crate) func_instance: FuncAddr,
    pub(crate) locals: Box<[RawWasmValue]>,
}

impl CallFrame {
    #[inline(always)]
    pub(crate) fn fetch_instr(&self, funcs: &[Function]) -> Instruction {
        // SAFETY: this is verified by the parser/validator
        let func = unsafe { funcs.get_unchecked(self.func_instance as usize) };
        let wasm_func = match func {
            Function::Wasm(wasm_func) => wasm_func,
            Function::Host(_) => {
                // SAFETY: we can never attempt to fetch an instruction for a Host function
                // because this type can only be constructed through an interface that requires
                // a WasmFunction.
                unsafe { unreachable_unchecked() }
            }
        };
        match wasm_func.instructions.get(self.instr_ptr) {
            Some(instr) => instr.clone(),
            None => {
                cold();
                panic!("Instruction pointer out of bounds");
            }
        }
    }

    /// Break to a block at the given index (relative to the current frame)
    /// Returns `None` if there is no block at the given index (e.g. if we need to return, this is handled by the caller)
    pub(crate) fn break_to(
        &mut self,
        break_to_relative: u32,
        values: &mut super::ValueStack,
        blocks: &mut super::BlockStack,
    ) -> Option<()> {
        let break_to = blocks.get_relative_to(break_to_relative, self.block_ptr)?;

        // instr_ptr points to the label instruction, but the next step
        // will increment it by 1 since we're changing the "current" instr_ptr
        match break_to.ty {
            BlockType::Loop => {
                // this is a loop, so we want to jump back to the start of the loop
                self.instr_ptr = break_to.instr_ptr;

                // We also want to push the params to the stack
                values.break_to(break_to.stack_ptr, break_to.params);

                // check if we're breaking to the loop
                if break_to_relative != 0 {
                    // we also want to trim the label stack to the loop (but not including the loop)
                    blocks.truncate(blocks.len() as u32 - break_to_relative);
                    return Some(());
                }
            }

            BlockType::Block | BlockType::If | BlockType::Else => {
                // this is a block, so we want to jump to the next instruction after the block ends
                // We also want to push the block's results to the stack
                values.break_to(break_to.stack_ptr, break_to.results);

                // (the inst_ptr will be incremented by 1 before the next instruction is executed)
                self.instr_ptr = break_to.instr_ptr + break_to.end_instr_offset as usize;

                // we also want to trim the label stack, including the block
                blocks.truncate(blocks.len() as u32 - (break_to_relative + 1));
            }
        }

        Some(())
    }

    #[inline(always)]
    pub(crate) fn new(
        wasm_func_addr: FuncAddr,
        wasm_func: &WasmFunction,
        params: impl ExactSizeIterator<Item = RawWasmValue>,
        block_ptr: u32,
    ) -> Self {
        let locals = {
            let total_size = wasm_func.locals.len() + params.len();
            let mut locals = Vec::new();
            locals.reserve_exact(total_size);
            locals.extend(params);
            locals.resize_with(total_size, RawWasmValue::default);
            locals.into_boxed_slice()
        };

        Self { instr_ptr: 0, func_instance: wasm_func_addr, locals, block_ptr }
    }

    #[inline(always)]
    pub(crate) fn set_local(&mut self, local_index: LocalAddr, value: RawWasmValue) {
        self.locals[local_index as usize] = value;
    }

    #[inline(always)]
    pub(crate) fn get_local(&self, local_index: LocalAddr) -> RawWasmValue {
        self.locals[local_index as usize]
    }

    #[inline(always)]
    pub(crate) fn instructions<'a>(&self, funcs: &'a [Function]) -> &'a [Instruction] {
        // SAFETY: this is verified by the parser/validator
        let func = unsafe { funcs.get_unchecked(self.func_instance as usize) };
        &match func {
            Function::Wasm(wasm_func) => wasm_func,
            Function::Host(_) => {
                // SAFETY: this is verified by the parser/validator
                unsafe { unreachable_unchecked() }
            }
        }
        .instructions
    }
}
