use std::io::{self, Read};

use num_enum::{IntoPrimitive, TryFromPrimitive};

use crate::module::LEB128Ext;
use crate::ValueType;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default, TryFromPrimitive, IntoPrimitive)]
#[repr(u8)]
pub enum BlockTypeInt {
    #[default]
    Void = 0x40,

    I32 = 0x7F,
    I64 = 0x7E,
    F32 = 0x7D,
    F64 = 0x7C,
    V128 = 0x7B,
    FuncRef = 0x70,
    ExternRef = 0x6F,
}

impl From<ValueType> for BlockTypeInt {
    fn from(value: ValueType) -> Self {
        Self::try_from_primitive(value.into()).unwrap()
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
#[repr(u8)]
pub enum BlockType {
    #[default]
    Void,

    I32,
    I64,
    F32,
    F64,
    V128,
    FuncRef,
    ExternRef,

    FuncType(usize),
}

impl From<BlockTypeInt> for BlockType {
    fn from(value: BlockTypeInt) -> Self {
        match value {
            BlockTypeInt::Void => Self::Void,
            BlockTypeInt::I32 => Self::I32,
            BlockTypeInt::I64 => Self::I64,
            BlockTypeInt::F32 => Self::F32,
            BlockTypeInt::F64 => Self::F64,
            BlockTypeInt::V128 => Self::V128,
            BlockTypeInt::FuncRef => Self::FuncRef,
            BlockTypeInt::ExternRef => Self::ExternRef,
        }
    }
}

pub fn parse_block_type<R: Read>(reader: &mut R) -> io::Result<BlockType> {
    // first try to read as signed int
    let index = reader.read_i32_leb()?;

    if index >= 0 {
        // if positive it is an index
        Ok(BlockType::FuncType(index as usize))
    } else {
        // if negative it was a concrete type already
        // random way of recovering the original first byte of an i32's LEB representation
        let val = (index as u8).wrapping_sub(0x80);
        Ok(BlockTypeInt::try_from_primitive(val)
            .map_err(io::Error::other)?
            .into())
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default, TryFromPrimitive)]
#[repr(u8)]
pub enum InstructionOpcode {
    // Control flow instructions
    #[default]
    Unreachable = 0x00,
    Nop = 0x01,
    Block = 0x02,
    Loop = 0x03,
    If = 0x04,
    Else = 0x05,
    // a = 0x06,
    // a = 0x07,
    // a = 0x08,
    // a = 0x09,
    // a = 0x0A,
    End = 0x0B,
    Br = 0x0C,
    BrIf = 0x0D,
    BrTable = 0x0E,
    Return = 0x0F,
    Call = 0x10,
    CallIndirect = 0x11,

    // Basic instructions
    Drop = 0x1A,
    //Select

    // Local/Global instructions

    // Memory instructions

    // Integer instructions
    ConstI32 = 0x41,
    ConstI64 = 0x42,
    ConstF32 = 0x43,
    ConstF64 = 0x44,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash, Default)]
pub enum Instruction {
    // Control flow instructions
    #[default]
    Unreachable,
    Nop,
    Block(BlockType),
    Loop(BlockType),
    If(BlockType),
    Else,
    End,
    Br(usize),
    BrIf(usize),
    BrTable(usize, usize),
    Return,
    Call(usize),
    CallIndirect(usize, bool),

    // Basic instructions
    Drop,
    //Select

    // Local/Global instructions

    // Memory instructions
    ConstI32(i32),
    ConstI64(i64),
    // stored as ints because Eq and Hash
    ConstF32(u32),
    ConstF64(u64),
}
