use std::io::{self, Read};

use byteorder::{ReadBytesExt, LE};
use num_enum::TryFromPrimitive;

use crate::module::{
    instruction::{parse_block_type, Instruction, InstructionOpcode},
    LEB128Ext,
};

#[derive(Debug, Clone, PartialEq, Eq, Hash)]
pub struct CodeSectionEntry {
    pub(crate) locals: (),
    pub(crate) instructions: Box<[Instruction]>,
}

pub fn parse_code_section<R: Read>(reader: &mut R) -> io::Result<Box<[CodeSectionEntry]>> {
    let codes_len = reader.read_u32_leb()?;
    let mut code_entries = Vec::new();
    code_entries.reserve_exact(codes_len as usize);

    for _ in 0..codes_len {
        let _body_len = reader.read_u32_leb()?;

        let num_locals = dbg!(reader.read_u32_leb()?);
        for _ in 0..num_locals {
            todo!("locals not yet supported")
        }

        let mut instructions = Vec::new();

        let mut depth = 1usize;
        while depth > 0 {
            let opcode = match InstructionOpcode::try_from_primitive(reader.read_u8()?) {
                Ok(opcode) => opcode,
                Err(err) => return Err(io::Error::other(err)),
            };

            instructions.push(match opcode {
                // Control flow instructions
                InstructionOpcode::Unreachable => Instruction::Unreachable,
                InstructionOpcode::Nop => Instruction::Nop,
                InstructionOpcode::Block => {
                    depth += 1;
                    Instruction::Block(parse_block_type(reader)?)
                }
                InstructionOpcode::Loop => {
                    depth += 1;
                    Instruction::Loop(parse_block_type(reader)?)
                }
                InstructionOpcode::If => {
                    depth += 1;
                    Instruction::If(parse_block_type(reader)?)
                }
                InstructionOpcode::Else => Instruction::Else,
                InstructionOpcode::End => {
                    depth -= 1;
                    Instruction::End
                }
                InstructionOpcode::Br => Instruction::Br(reader.read_u32_leb()? as usize),
                InstructionOpcode::BrIf => Instruction::BrIf(reader.read_u32_leb()? as usize),
                // InstructionOpcode::BrTable => Instruction::BrTable(
                //     reader.read_u32_leb()? as usize,
                //     reader.read_u32_leb()? as usize,
                // ),
                InstructionOpcode::BrTable => todo!("br_table not supported"),
                InstructionOpcode::Return => Instruction::Return,
                InstructionOpcode::Call => Instruction::Call(reader.read_u32_leb()? as usize),
                InstructionOpcode::CallIndirect => todo!("call_indirect not supported"),

                // Basic instructions
                InstructionOpcode::Drop => Instruction::Drop,

                // Local/Global instructions

                // Memory instructions

                // Integer instructions
                InstructionOpcode::ConstI32 => Instruction::ConstI32(reader.read_i32_leb()?),
                InstructionOpcode::ConstI64 => Instruction::ConstI64(reader.read_i64_leb()?),
                InstructionOpcode::ConstF32 => Instruction::ConstF32(reader.read_u32::<LE>()?),
                InstructionOpcode::ConstF64 => Instruction::ConstF64(reader.read_u64::<LE>()?),
            });
        }

        // and now we pray that all instructions of this function body have been read

        code_entries.push(CodeSectionEntry {
            locals: (),
            instructions: instructions.into(),
        });
    }

    Ok(code_entries.into_boxed_slice())
}
