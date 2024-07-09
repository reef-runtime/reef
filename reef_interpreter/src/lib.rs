// #![no_std]
#![warn(missing_docs, missing_debug_implementations, rust_2018_idioms, unreachable_pub)]

//! A tiny WebAssembly Runtime written in Rust
//!
//! This crate provides a minimal WebAssembly runtime for executing WebAssembly modules.
//! It currently supports all features of the WebAssembly MVP specification and is
//! designed to be easy to use and integrate in other projects.
//!
//! ## Features
//!- **`std`**\
//!  Enables the use of `std` and `std::io` for parsing from files and streams. This is enabled by default.
//!
//! ## Getting Started
//! The easiest way to get started is to use the [`Module::parse_bytes`] function to load a
//! WebAssembly module from bytes. This will parse the module and validate it, returning
//! a [`Module`] that can be used to instantiate the module.
//!
//! ## Imports
//!
//! To provide imports to a module, you can use the [`Imports`] struct.
//! This struct allows you to register custom functions, globals, memories, tables,
//! and other modules to be linked into the module when it is instantiated.
//!
//! See the [`Imports`] documentation for more information.

extern crate alloc;

pub mod error;
pub mod exec;
pub mod func;
pub mod imports;
mod instance;
mod module;
mod parser;
pub mod reference;
mod runtime;
mod store;
pub mod types;

pub use error::Error;
pub use instance::Instance;
pub use module::parse_bytes;
pub use types::Module;

pub(crate) const CALL_STACK_SIZE: usize = 1024;

/// Max Wasm page size
pub const PAGE_SIZE: usize = 65536;
/// Max number of pages for a Wasm module
pub const MAX_PAGES: usize = 65536;
const MAX_SIZE: u64 = PAGE_SIZE as u64 * MAX_PAGES as u64;

#[cold]
pub(crate) fn cold() {}

pub(crate) fn unlikely(b: bool) -> bool {
    if b {
        cold()
    };
    b
}

pub(crate) trait VecExt<T> {
    fn add(&mut self, element: T) -> usize;

    fn get_or<E, F>(&self, index: usize, err: F) -> Result<&T, E>
    where
        F: FnOnce() -> E;
    fn get_mut_or<E, F>(&mut self, index: usize, err: F) -> Result<&mut T, E>
    where
        F: FnOnce() -> E;

    fn get_or_instance(&self, index: u32, name: &str) -> Result<&T, error::Error>;
    fn get_mut_or_instance(&mut self, index: u32, name: &str) -> Result<&mut T, error::Error>;
}
impl<T> VecExt<T> for alloc::vec::Vec<T> {
    fn add(&mut self, value: T) -> usize {
        self.push(value);
        self.len() - 1
    }

    fn get_or<E, F>(&self, index: usize, err: F) -> Result<&T, E>
    where
        F: FnOnce() -> E,
    {
        self.get(index).ok_or_else(err)
    }

    fn get_mut_or<E, F>(&mut self, index: usize, err: F) -> Result<&mut T, E>
    where
        F: FnOnce() -> E,
    {
        self.get_mut(index).ok_or_else(err)
    }

    fn get_or_instance(&self, index: u32, name: &str) -> Result<&T, error::Error> {
        self.get_or(index as usize, || Instance::not_found_error(name))
    }
    fn get_mut_or_instance(&mut self, index: u32, name: &str) -> Result<&mut T, error::Error> {
        self.get_mut_or(index as usize, || Instance::not_found_error(name))
    }
}
