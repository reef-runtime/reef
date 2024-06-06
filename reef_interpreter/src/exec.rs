//! Modules for types related to controlling the execution of Wasm

use alloc::vec::Vec;
use core::mem::take;

use rkyv::{
    ser::{
        serializers::{AlignedSerializer, CompositeSerializer, HeapScratch, SharedSerializeMap},
        Serializer,
    },
    AlignedVec,
};

use crate::error::Result;
use crate::func::{FromWasmValueTuple, FuncHandle};
use crate::runtime::{RawWasmValue, Stack};
use crate::types::value::WasmValue;

/// Retuened by [`run`](ExecHandle::run) to indicate if the function finsihed execution with the given max_cycles
#[derive(Debug)]
pub enum CallResult {
    /// Execution finished and the resulting function return is included
    Done(Vec<WasmValue>),
    /// Execution has not finished and `run` has to be called again
    Incomplete,
}

/// Handle to a running execution context of a Wasm function
#[derive(Debug)]
pub struct ExecHandle {
    pub(crate) func_handle: FuncHandle,
    pub(crate) stack: Stack,
}

impl ExecHandle {
    /// Make progress on the execution of the started Wasm function. `max_cycles` instructions will be executed.
    pub fn run(&mut self, max_cycles: usize) -> Result<CallResult> {
        let runtime = crate::runtime::interpreter::Interpreter {};
        if !runtime.exec(&mut self.func_handle.instance, &mut self.stack, max_cycles)? {
            return Ok(CallResult::Incomplete);
        }

        // Once the function returns:
        let result_m = self.func_handle.ty.results.len();

        // 1. Assert: m values are on the top of the stack (Ensured by validation)
        assert!(self.stack.values.len() >= result_m);

        // 2. Pop m values from the stack
        let res = self.stack.values.last_n(result_m)?;

        // The values are returned as the results of the invocation.
        Ok(CallResult::Done(
            res.iter().zip(self.func_handle.ty.results.iter()).map(|(v, ty)| v.attach_type(*ty)).collect(),
        ))
    }

    /// Take the current execution state and serialize it
    pub fn serialize(&mut self, buf: AlignedVec) -> Result<AlignedVec> {
        let memory = &mut self.func_handle.instance.memories[0];
        let globals = self.func_handle.instance.globals.iter().map(|g| g.value).collect();
        let data = SerializationState { stack: take(&mut self.stack), memory: take(&mut memory.data), globals };

        let mut serializer = CompositeSerializer::new(
            AlignedSerializer::new(buf),
            HeapScratch::<0x1000>::new(),
            SharedSerializeMap::new(),
        );
        serializer.serialize_value(&data).expect("Failed to serialize state");

        memory.data = data.memory;
        self.stack = data.stack;

        Ok(serializer.into_serializer().into_inner())
    }
}

/// Like [`CallResult`], but typed
#[derive(Debug)]
pub enum CallResultTyped<R: FromWasmValueTuple> {
    /// See [`CallResult::Done`]
    Done(R),
    /// See [`CallResult::Incomplete`]
    Incomplete,
}

/// [`ExecHandle`] but typed
#[derive(Debug)]
pub struct ExecHandleTyped<R: FromWasmValueTuple> {
    pub(crate) exec_handle: ExecHandle,
    pub(crate) _marker: core::marker::PhantomData<R>,
}

impl<R: FromWasmValueTuple> ExecHandleTyped<R> {
    /// See [`ExecHandle::run`]
    pub fn run(&mut self, max_cycles: usize) -> Result<CallResultTyped<R>> {
        // Call the underlying WASM function
        let result = self.exec_handle.run(max_cycles)?;

        Ok(match result {
            CallResult::Done(values) => CallResultTyped::Done(R::from_wasm_value_tuple(&values)?),
            CallResult::Incomplete => CallResultTyped::Incomplete,
        })
    }

    /// See [`ExecHandle::serialize`]
    pub fn serialize(&mut self, buf: AlignedVec) -> Result<AlignedVec> {
        self.exec_handle.serialize(buf)
    }
}

#[derive(Debug, Clone, PartialEq, Eq, rkyv::Archive, rkyv::Serialize, rkyv::Deserialize)]
#[archive(check_bytes)]
pub(crate) struct SerializationState {
    pub(crate) stack: Stack,
    pub(crate) memory: Vec<u8>,
    pub(crate) globals: Vec<RawWasmValue>,
}
