use crate::module::{instruction::BlockType, ExternalKind, Module};

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
    label: usize,
    limit: usize,
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
                // TODO: find some efficient way of finding correct labels and set this one properly
                label: 0,
                limit: 0,
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
}
