use num_enum::{IntoPrimitive, TryFromPrimitive, TryFromPrimitiveError};

pub mod execution;
pub mod module;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, TryFromPrimitive, IntoPrimitive)]
#[repr(u8)]
pub enum ValueType {
    #[default]
    I32 = 0x7F,
    I64 = 0x7E,
    F32 = 0x7D,
    F64 = 0x7C,
    V128 = 0x7B,
    FuncRef = 0x70,
    ExternRef = 0x6F,
}

impl From<NumberType> for ValueType {
    fn from(value: NumberType) -> Self {
        Self::try_from_primitive(value.into()).unwrap()
    }
}

impl From<VectorType> for ValueType {
    fn from(value: VectorType) -> Self {
        Self::try_from_primitive(value.into()).unwrap()
    }
}

impl From<ReferenceType> for ValueType {
    fn from(value: ReferenceType) -> Self {
        Self::try_from_primitive(value.into()).unwrap()
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, TryFromPrimitive, IntoPrimitive)]
#[repr(u8)]
pub enum NumberType {
    #[default]
    I32 = 0x7F,
    I64 = 0x7E,
    F32 = 0x7D,
    F64 = 0x7C,
}

impl TryFrom<ValueType> for NumberType {
    type Error = TryFromPrimitiveError<Self>;
    fn try_from(value: ValueType) -> Result<Self, Self::Error> {
        Self::try_from_primitive(value.into())
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, TryFromPrimitive, IntoPrimitive)]
#[repr(u8)]
pub enum VectorType {
    #[default]
    V128 = 0x7B,
}

impl TryFrom<ValueType> for VectorType {
    type Error = TryFromPrimitiveError<Self>;
    fn try_from(value: ValueType) -> Result<Self, Self::Error> {
        Self::try_from_primitive(value.into())
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, TryFromPrimitive, IntoPrimitive)]
#[repr(u8)]
pub enum ReferenceType {
    #[default]
    FuncRef = 0x70,
    ExternRef = 0x6F,
}

impl TryFrom<ValueType> for ReferenceType {
    type Error = TryFromPrimitiveError<Self>;
    fn try_from(value: ValueType) -> Result<Self, Self::Error> {
        Self::try_from_primitive(value.into())
    }
}
