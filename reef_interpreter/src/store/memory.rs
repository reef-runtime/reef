use alloc::{vec, vec::Vec};

use crate::error::{Error, Result, Trap};
use crate::types::MemoryType;
use crate::{MAX_PAGES, MAX_SIZE, PAGE_SIZE};

/// A WebAssembly Memory Instance
///
/// See <https://webassembly.github.io/spec/core/exec/runtime.html#memory-instances>
#[derive(Debug)]
pub(crate) struct MemoryInstance {
    pub(crate) kind: MemoryType,
    pub(crate) data: Vec<u8>,
    pub(crate) page_count: usize,
}

impl MemoryInstance {
    pub(crate) fn new(kind: MemoryType) -> Self {
        assert!(kind.page_count_initial <= kind.page_count_max.unwrap_or(MAX_PAGES as u64));

        Self {
            kind,
            data: vec![0; PAGE_SIZE * kind.page_count_initial as usize],
            page_count: kind.page_count_initial as usize,
        }
    }

    #[inline(never)]
    #[cold]
    fn trap_oob(&self, addr: usize, len: usize) -> Error {
        Error::Trap(Trap::MemoryOutOfBounds { offset: addr, len, max: self.data.len() })
    }

    pub(crate) fn store(&mut self, addr: usize, len: usize, data: &[u8]) -> Result<()> {
        let Some(end) = addr.checked_add(len) else {
            return Err(self.trap_oob(addr, data.len()));
        };

        if end > self.data.len() || end < addr {
            return Err(self.trap_oob(addr, data.len()));
        }

        self.data[addr..end].copy_from_slice(data);
        Ok(())
    }

    pub(crate) fn max_pages(&self) -> usize {
        self.kind.page_count_max.unwrap_or(MAX_PAGES as u64) as usize
    }

    pub(crate) fn load(&self, addr: usize, len: usize) -> Result<&[u8]> {
        let Some(end) = addr.checked_add(len) else {
            return Err(self.trap_oob(addr, len));
        };

        if end > self.data.len() || end < addr {
            return Err(self.trap_oob(addr, len));
        }

        Ok(&self.data[addr..end])
    }

    // this is a workaround since we can't use generic const expressions yet (https://github.com/rust-lang/rust/issues/76560)
    pub(crate) fn load_as<const SIZE: usize, T: MemLoadable<SIZE>>(&self, addr: usize) -> Result<T> {
        let Some(end) = addr.checked_add(SIZE) else {
            return Err(self.trap_oob(addr, SIZE));
        };

        if end > self.data.len() {
            return Err(self.trap_oob(addr, SIZE));
        }
        let val = T::from_le_bytes(match self.data[addr..end].try_into() {
            Ok(bytes) => bytes,
            Err(_) => unreachable!("checked bounds above"),
        });

        Ok(val)
    }

    #[inline]
    pub(crate) fn page_count(&self) -> usize {
        self.page_count
    }

    pub(crate) fn fill(&mut self, addr: usize, len: usize, val: u8) -> Result<()> {
        let end = addr.checked_add(len).ok_or_else(|| self.trap_oob(addr, len))?;
        if end > self.data.len() {
            return Err(self.trap_oob(addr, len));
        }

        self.data[addr..end].fill(val);
        Ok(())
    }

    // needed for copy between different memories
    //
    // pub(crate) fn copy_from_slice(&mut self, dst: usize, src: &[u8]) -> Result<()> {
    //     let end = dst.checked_add(src.len()).ok_or_else(|| self.trap_oob(dst, src.len()))?;
    //     if end > self.data.len() {
    //         return Err(self.trap_oob(dst, src.len()));
    //     }

    //     self.data[dst..end].copy_from_slice(src);
    //     Ok(())
    // }

    pub(crate) fn copy_within(&mut self, dst: usize, src: usize, len: usize) -> Result<()> {
        // Calculate the end of the source slice
        let src_end = src.checked_add(len).ok_or_else(|| self.trap_oob(src, len))?;
        if src_end > self.data.len() {
            return Err(self.trap_oob(src, len));
        }

        // Calculate the end of the destination slice
        let dst_end = dst.checked_add(len).ok_or_else(|| self.trap_oob(dst, len))?;
        if dst_end > self.data.len() {
            return Err(self.trap_oob(dst, len));
        }

        // Perform the copy
        self.data.copy_within(src..src_end, dst);
        Ok(())
    }

    pub(crate) fn grow(&mut self, pages_delta: i32) -> Option<i32> {
        let current_pages = self.page_count();
        let new_pages = current_pages as i64 + pages_delta as i64;

        if new_pages < 0 || new_pages > MAX_PAGES as i64 {
            return None;
        }

        if new_pages as usize > self.max_pages() {
            return None;
        }

        let new_size = new_pages as usize * PAGE_SIZE;
        if new_size as u64 > MAX_SIZE {
            return None;
        }

        // Zero initialize the new pages
        self.data.resize(new_size, 0);
        self.page_count = new_pages as usize;
        debug_assert!(current_pages <= i32::MAX as usize, "page count should never be greater than i32::MAX");
        Some(current_pages as i32)
    }
}

/// A trait for types that can be loaded from memory
pub(crate) trait MemLoadable<const T: usize>: Sized + Copy {
    /// Load a value from memory
    fn from_le_bytes(bytes: [u8; T]) -> Self;
}

macro_rules! impl_mem_loadable_for_primitive {
    ($($type:ty, $size:expr),*) => {
        $(
            impl MemLoadable<$size> for $type {
                #[inline(always)]
                fn from_le_bytes(bytes: [u8; $size]) -> Self {
                    <$type>::from_le_bytes(bytes)
                }
            }
        )*
    }
}

impl_mem_loadable_for_primitive!(
    u8, 1, i8, 1, u16, 2, i16, 2, u32, 4, i32, 4, f32, 4, u64, 8, i64, 8, f64, 8, u128, 16, i128, 16
);
