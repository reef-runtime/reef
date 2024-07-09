//! Types related to getting handles for functions in a Wasm module

use alloc::{
    boxed::Box,
    format,
    string::{String, ToString},
    vec,
    vec::Vec,
};

use crate::error::{Error, Result};
use crate::exec::{ExecHandle, ExecHandleTyped};
use crate::imports::Function;
use crate::instance::Instance;
use crate::runtime::{CallFrame, RawWasmValue, Stack};
use crate::types::{
    value::{ValType, WasmValue},
    FuncType,
};
use crate::{unlikely, VecExt};

#[derive(Debug)]
/// A function handle
pub struct FuncHandle {
    pub(crate) instance: Instance,

    pub(crate) addr: u32,
    pub(crate) ty: FuncType,

    /// The name of the function, if it has one
    pub name: Option<String>,
}

impl FuncHandle {
    /// Start or resume execution of function
    pub fn call(self, params: Vec<WasmValue>, stack: Option<Stack>) -> Result<ExecHandle> {
        let func_ty = &self.ty;

        if unlikely(func_ty.params.len() != params.len()) {
            return Err(Error::Other(format!(
                "param count mismatch: expected {}, got {}",
                func_ty.params.len(),
                params.len()
            )));
        }

        if !(func_ty.params.iter().zip(&params).all(|(ty, param)| ty == &param.val_type())) {
            return Err(Error::Other("Type mismatch".into()));
        }

        let func = self.instance.funcs.get_or_instance(self.addr, "function")?;

        let stack = match stack {
            Some(stack) => stack,
            None => match &func {
                Function::Wasm(wasm_func) => {
                    let call_frame_params = params.iter().map(|v| RawWasmValue::from(*v));
                    let call_frame = CallFrame::new(self.addr, wasm_func, call_frame_params, 0);
                    Stack::new(call_frame)
                }
                Function::Host(_) => return Err(Error::Other("Can't call Host function directly".to_string())),
            },
        };

        Ok(ExecHandle { func_handle: self, stack })
    }
}

/// A typed function handle
#[derive(Debug)]
pub struct FuncHandleTyped<P, R> {
    /// The underlying function handle
    pub func: FuncHandle,
    pub(crate) _marker: core::marker::PhantomData<(P, R)>,
}

/// Things that can be converted to WasmValues
pub trait IntoWasmValueTuple {
    /// Do the conversion
    fn into_wasm_value_tuple(self) -> Vec<WasmValue>;
}

/// Things that can constructed from WasmValues
pub trait FromWasmValueTuple {
    /// Do the conversion
    fn from_wasm_value_tuple(values: &[WasmValue]) -> Result<Self>
    where
        Self: Sized;
}

impl<P: IntoWasmValueTuple, R: FromWasmValueTuple> FuncHandleTyped<P, R> {
    /// See [`FuncHandle::call`]
    pub fn call(self, params: P, stack: Option<Stack>) -> Result<ExecHandleTyped<R>> {
        let exec_handle = self.func.call(params.into_wasm_value_tuple(), stack)?;

        Ok(ExecHandleTyped { exec_handle, _marker: Default::default() })
    }
}

macro_rules! impl_into_wasm_value_tuple {
    ($($T:ident),*) => {
        impl<$($T),*> IntoWasmValueTuple for ($($T,)*)
        where
            $($T: Into<WasmValue>),*
        {
            #[allow(non_snake_case)]
            #[inline]
            fn into_wasm_value_tuple(self) -> Vec<WasmValue> {
                let ($($T,)*) = self;
                vec![$($T.into(),)*]
            }
        }
    }
}

macro_rules! impl_into_wasm_value_tuple_single {
    ($T:ident) => {
        impl IntoWasmValueTuple for $T {
            #[inline]
            fn into_wasm_value_tuple(self) -> Vec<WasmValue> {
                vec![self.into()]
            }
        }
    };
}

macro_rules! impl_from_wasm_value_tuple {
    ($($T:ident),*) => {
        impl<$($T),*> FromWasmValueTuple for ($($T,)*)
        where
            $($T: TryFrom<WasmValue, Error = ()>),*
        {
            #[inline]
            fn from_wasm_value_tuple(values: &[WasmValue]) -> Result<Self> {
                #[allow(unused_variables, unused_mut)]
                let mut iter = values.iter();

                Ok((
                    $(
                        $T::try_from(
                            *iter.next()
                            .ok_or(Error::Other("Not enough values in WasmValue vector".to_string()))?
                        )
                        .map_err(|e| Error::Other(format!("FromWasmValueTuple: Could not convert WasmValue to expected type: {:?}", e,
                    )))?,
                    )*
                ))
            }
        }
    }
}

macro_rules! impl_from_wasm_value_tuple_single {
    ($T:ident) => {
        impl FromWasmValueTuple for $T {
            #[inline]
            fn from_wasm_value_tuple(values: &[WasmValue]) -> Result<Self> {
                #[allow(unused_variables, unused_mut)]
                let mut iter = values.iter();
                $T::try_from(*iter.next().ok_or(Error::Other("Not enough values in WasmValue vector".to_string()))?)
                    .map_err(|e| {
                        Error::Other(format!(
                            "FromWasmValueTupleSingle: Could not convert WasmValue to expected type: {:?}",
                            e
                        ))
                    })
            }
        }
    };
}

/// Types that can be constructed from a tuple
pub trait ValTypesFromTuple {
    /// Do the conversion
    fn val_types() -> Box<[ValType]>;
}

/// Types that can be turned into a tuple
pub trait ToValType {
    /// Do the conversion
    fn to_val_type() -> ValType;
}

impl ToValType for i32 {
    fn to_val_type() -> ValType {
        ValType::I32
    }
}

impl ToValType for i64 {
    fn to_val_type() -> ValType {
        ValType::I64
    }
}

impl ToValType for f32 {
    fn to_val_type() -> ValType {
        ValType::F32
    }
}

impl ToValType for f64 {
    fn to_val_type() -> ValType {
        ValType::F64
    }
}

macro_rules! impl_val_types_from_tuple {
    ($($t:ident),+) => {
        impl<$($t),+> ValTypesFromTuple for ($($t,)+)
        where
            $($t: ToValType,)+
        {
            #[inline]
            fn val_types() -> Box<[ValType]> {
                Box::new([$($t::to_val_type(),)+])
            }
        }
    };
}

impl ValTypesFromTuple for () {
    #[inline]
    fn val_types() -> Box<[ValType]> {
        Box::new([])
    }
}

impl<T: ToValType> ValTypesFromTuple for T {
    #[inline]
    fn val_types() -> Box<[ValType]> {
        Box::new([T::to_val_type()])
    }
}

impl_from_wasm_value_tuple_single!(i32);
impl_from_wasm_value_tuple_single!(i64);
impl_from_wasm_value_tuple_single!(f32);
impl_from_wasm_value_tuple_single!(f64);

impl_into_wasm_value_tuple_single!(i32);
impl_into_wasm_value_tuple_single!(i64);
impl_into_wasm_value_tuple_single!(f32);
impl_into_wasm_value_tuple_single!(f64);

impl_val_types_from_tuple!(T1);
impl_val_types_from_tuple!(T1, T2);
impl_val_types_from_tuple!(T1, T2, T3);
impl_val_types_from_tuple!(T1, T2, T3, T4);
impl_val_types_from_tuple!(T1, T2, T3, T4, T5);
impl_val_types_from_tuple!(T1, T2, T3, T4, T5, T6);

impl_from_wasm_value_tuple!();
impl_from_wasm_value_tuple!(T1);
impl_from_wasm_value_tuple!(T1, T2);
impl_from_wasm_value_tuple!(T1, T2, T3);
impl_from_wasm_value_tuple!(T1, T2, T3, T4);
impl_from_wasm_value_tuple!(T1, T2, T3, T4, T5);
impl_from_wasm_value_tuple!(T1, T2, T3, T4, T5, T6);

impl_into_wasm_value_tuple!();
impl_into_wasm_value_tuple!(T1);
impl_into_wasm_value_tuple!(T1, T2);
impl_into_wasm_value_tuple!(T1, T2, T3);
impl_into_wasm_value_tuple!(T1, T2, T3, T4);
impl_into_wasm_value_tuple!(T1, T2, T3, T4, T5);
impl_into_wasm_value_tuple!(T1, T2, T3, T4, T5, T6);
