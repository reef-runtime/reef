//! References to parts of instatiated Wasm modules

use alloc::{
    ffi::CString,
    string::{String, ToString},
    vec::Vec,
};
use core::ffi::CStr;

use crate::error::{Error, Result};
use crate::store::{global::GlobalInstance, memory::MemoryInstance};
use crate::types::value::WasmValue;

// This module essentially contains the public APIs to interact with the data stored in the store

/// A reference to a memory instance
#[derive(Debug)]
pub struct MemoryRef<'m> {
    pub(crate) instance: &'m MemoryInstance,
}

/// A borrowed reference to a memory instance
#[derive(Debug)]
pub struct MemoryRefMut<'m> {
    pub(crate) instance: &'m mut MemoryInstance,
}

impl<'a> MemoryRefLoad for MemoryRef<'a> {
    /// Load a slice of memory
    fn load(&self, offset: usize, len: usize) -> Result<&[u8]> {
        self.instance.load(offset, len)
    }
}

impl<'a> MemoryRefLoad for MemoryRefMut<'a> {
    /// Load a slice of memory
    fn load(&self, offset: usize, len: usize) -> Result<&[u8]> {
        self.instance.load(offset, len)
    }
}

impl MemoryRef<'_> {
    /// Load a slice of memory
    pub fn load(&self, offset: usize, len: usize) -> Result<&[u8]> {
        self.instance.load(offset, len)
    }

    /// Load a slice of memory as a vector
    pub fn load_vec(&self, offset: usize, len: usize) -> Result<Vec<u8>> {
        self.load(offset, len).map(|x| x.to_vec())
    }
}

impl MemoryRefMut<'_> {
    /// Load a slice of memory
    pub fn load(&self, offset: usize, len: usize) -> Result<&[u8]> {
        self.instance.load(offset, len)
    }

    /// Load a slice of memory as a vector
    pub fn load_vec(&self, offset: usize, len: usize) -> Result<Vec<u8>> {
        self.load(offset, len).map(|x| x.to_vec())
    }

    /// Grow the memory by the given number of pages
    pub fn grow(&mut self, delta_pages: i32) -> Option<i32> {
        self.instance.grow(delta_pages)
    }

    /// Get the current size of the memory in pages
    pub fn page_count(&mut self) -> usize {
        self.instance.page_count()
    }

    /// Copy a slice of memory to another place in memory
    pub fn copy_within(&mut self, src: usize, dst: usize, len: usize) -> Result<()> {
        self.instance.copy_within(src, dst, len)
    }

    /// Fill a slice of memory with a value
    pub fn fill(&mut self, offset: usize, len: usize, val: u8) -> Result<()> {
        self.instance.fill(offset, len, val)
    }

    /// Store a slice of memory
    pub fn store(&mut self, offset: usize, len: usize, data: &[u8]) -> Result<()> {
        self.instance.store(offset, len, data)
    }

    /// Set pages ignored during serialization
    pub fn set_ignored_page_region(&mut self, page: usize, count: usize) {
        self.instance.ignored_page_region = (page, page + count);
    }

    /// Copy data into the region ignored by serialization
    pub fn copy_into_ignored_page_region(&mut self, data: &[u8]) {
        let start = self.instance.ignored_page_region.0 * crate::PAGE_SIZE;
        let end = self.instance.ignored_page_region.1 * crate::PAGE_SIZE;
        self.instance.data[start..end].copy_from_slice(data);
    }
}

#[doc(hidden)]
pub trait MemoryRefLoad {
    fn load(&self, offset: usize, len: usize) -> Result<&[u8]>;
    fn load_vec(&self, offset: usize, len: usize) -> Result<Vec<u8>> {
        self.load(offset, len).map(|x| x.to_vec())
    }
}

/// Convenience methods for loading strings from memory
pub trait MemoryStringExt: MemoryRefLoad {
    /// Load a C-style string from memory
    fn load_cstr(&self, offset: usize, len: usize) -> Result<&CStr> {
        let bytes = self.load(offset, len)?;
        CStr::from_bytes_with_nul(bytes).map_err(|_| Error::Other("Invalid C-style string".to_string()))
    }

    /// Load a C-style string from memory, stopping at the first nul byte
    fn load_cstr_until_nul(&self, offset: usize, max_len: usize) -> Result<&CStr> {
        let bytes = self.load(offset, max_len)?;
        CStr::from_bytes_until_nul(bytes).map_err(|_| Error::Other("Invalid C-style string".to_string()))
    }

    /// Load a UTF-8 string from memory
    fn load_string(&self, offset: usize, len: usize) -> Result<String> {
        let bytes = self.load(offset, len)?;
        String::from_utf8(bytes.to_vec()).map_err(|_| Error::Other("Invalid UTF-8 string".to_string()))
    }

    /// Load a C-style string from memory
    fn load_cstring(&self, offset: usize, len: usize) -> Result<CString> {
        Ok(CString::from(self.load_cstr(offset, len)?))
    }

    /// Load a C-style string from memory, stopping at the first nul byte
    fn load_cstring_until_nul(&self, offset: usize, max_len: usize) -> Result<CString> {
        Ok(CString::from(self.load_cstr_until_nul(offset, max_len)?))
    }

    /// Load a JavaScript-style utf-16 string from memory
    fn load_js_string(&self, offset: usize, len: usize) -> Result<String> {
        let bytes = self.load(offset, len)?;
        let mut string = String::new();
        for i in 0..(len / 2) {
            let c = u16::from_le_bytes([bytes[i * 2], bytes[i * 2 + 1]]);
            string.push(char::from_u32(c as u32).ok_or_else(|| Error::Other("Invalid UTF-16 string".to_string()))?);
        }
        Ok(string)
    }
}

impl MemoryStringExt for MemoryRef<'_> {}
impl MemoryStringExt for MemoryRefMut<'_> {}

/// A reference to a global instance
#[derive(Debug)]
pub struct GlobalRef<'i> {
    pub(crate) instance: &'i mut GlobalInstance,
}

impl<'i> GlobalRef<'i> {
    /// Get the value of the global
    pub fn get(&self) -> WasmValue {
        self.instance.get()
    }

    /// Set the value of the global
    pub fn set(&mut self, val: WasmValue) -> Result<()> {
        self.instance.set(val)
    }
}
