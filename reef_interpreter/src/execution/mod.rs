use crate::module::{
    instruction::{BlockType, Instruction},
    ExternalKind, Module,
};

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
pub struct ExecutionContext {
    module: Module,
    state: ExecutionState,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct ExecutionState {
    call_stack: Vec<ExecutionFrame>,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct ExecutionFrame {
    function_index: usize,
    current_instruction: usize,
    control_stack: Vec<ExecutionControl>,
    value_stack: Vec<u64>,
    locals: Vec<u64>,
}

#[derive(Debug, Clone, Hash, PartialEq, Eq)]
struct ExecutionControl {
    // label: usize,
    // limit: usize,
    jump_target: usize,
    block_type: BlockType,
}

impl ExecutionContext {
    pub fn start(module: Module, function: &str, _parameters: ()) -> Result<Self, ExecutionError> {
        // search for function
        let export = module
            .export_section
            .iter()
            .find(|export| &*export.export_name == function);
        let Some(export) = export else {
            return Err(ExecutionError::FunctionNotFound(function.into()));
        };
        if export.export_kind != ExternalKind::Function {
            return Err(ExecutionError::InvalidExport);
        }
        let function_index = export.export_index;
        let Some(function) = module.function_section.get(function_index) else {
            return Err(ExecutionError::OutOfBounds(function_index));
        };

        let frame = ExecutionFrame {
            function_index,
            current_instruction: 0,
            control_stack: vec![ExecutionControl {
                jump_target: Self::scan_for_target(
                    &module.code_section[function_index].instructions,
                    0,
                ),
                block_type: BlockType::FuncType(function.signature_index),
            }],
            value_stack: vec![],
            locals: vec![],
        };

        Ok(Self {
            module,
            state: ExecutionState {
                call_stack: vec![frame],
            },
        })
    }

    pub(crate) fn scan_for_target(instructions: &[Instruction], current: usize) -> usize {
        let mut i = current;

        let mut depth = 1usize;
        while depth > 0 {
            match instructions[i] {
                Instruction::Block(_) => {
                    depth += 1;
                }
                Instruction::Loop(_) => {
                    depth += 1;
                }
                Instruction::If(_) => {
                    depth += 1;
                }
                Instruction::Else => {}
                Instruction::End => {
                    depth -= 1;
                }
                _ => {}
            }

            i += 1;
        }

        i - 1
    }

    pub fn step(&mut self) -> Result<Option<()>, ExecutionError> {
        let current_frame = self.state.call_stack.last_mut().unwrap();

        // load current instruction
        let current_function = &self.module.code_section[current_frame.function_index];
        let current_instruction = &current_function.instructions[current_frame.current_instruction];

        // increment instruction pointer
        current_frame.current_instruction += 1;

        match current_instruction {
            // Control flow instructions
            Instruction::Unreachable => return Err(ExecutionError::UnreachableInstruction),
            Instruction::Nop => {}
            // Instruction::Block(BlockType),
            // Instruction::Loop(BlockType),
            // Instruction::If(BlockType),
            // Instruction::Else,
            Instruction::End => {
                // pop entry from control stack
                current_frame.control_stack.pop();
            }
            // Instruction::Br(usize),
            // Instruction::BrIf(usize),
            // Instruction::BrTable(usize, usize),
            // Instruction::Return,
            // Instruction::Call(usize),
            // Instruction::CallIndirect(usize, bool),

            // Basic instructions
            // Instruction::Drop,
            //Select

            // Local/Global instructions

            // Memory instructions

            // Integer instructions
            // Instruction::ConstI32(i32),
            // Instruction::ConstI64(i64),
            // stored as ints because Eq and Hash
            // Instruction::ConstF32(u32),
            // Instruction::ConstF64(u64),
            _ => todo!("Unimplemented instruction {current_instruction:?}"),
        }

        // if control stack is empty we need to return from the current function

        let frame_is_empty = current_frame.control_stack.is_empty();
        // drop(current_frame);
        if frame_is_empty {
            let frame = self.state.call_stack.pop();

            // TODO pop return values from stack

            // check if we need to return to other function or outer env
            match self.state.call_stack.last_mut() {
                Some(prev_frame) => {
                    todo!("return not yet supported");
                }
                None => {
                    // TODO: return resuts
                    return Ok(Some(()));
                }
            }
        }

        Ok(None)
    }

    pub fn into_module(self) -> Module {
        self.module
    }
}

#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord, thiserror::Error)]
pub enum ExecutionError {
    #[error("The function `{0}` is not available")]
    FunctionNotFound(String),
    #[error("This export can not be executed")]
    InvalidExport,
    #[error("Index `{0}` is out of bounds")]
    OutOfBounds(usize),

    #[error("`unreachable` instruction hit")]
    UnreachableInstruction,
}
