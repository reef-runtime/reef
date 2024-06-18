use alloc::{vec, vec::Vec};

use crate::error::{Error, Result, Trap};
use crate::types::MemoryType;
use crate::{MAX_PAGES, MAX_SIZE, PAGE_SIZE};

/// A WebAssembly Memory Instance
///
/// See <https://webassembly.github.io/spec/core/exec/runtime.html#memory-instances>
#[derive(Debug, Clone, PartialEq, Eq)]
pub(crate) struct MemoryInstance {
    pub(crate) kind: MemoryType,
    pub(crate) page_count: usize,
    /// ignored during serialization
    pub(crate) ignored_page_region: (usize, usize),
    pub(crate) data: Vec<u8>,
}

impl MemoryInstance {
    pub(crate) fn new(kind: MemoryType) -> Self {
        assert!(kind.page_count_initial <= kind.page_count_max.unwrap_or(MAX_PAGES as u64));

        Self {
            kind,
            data: vec![0; PAGE_SIZE * kind.page_count_initial as usize],
            page_count: kind.page_count_initial as usize,
            ignored_page_region: (0, 0),
        }
    }

    #[inline(never)]
    #[cold]
    fn trap_oob(&self, addr: usize, len: usize) -> Error {
        Error::Trap(Trap::MemoryOutOfBounds { offset: addr, len, max: self.data.len() })
    }

    #[inline]
    pub(crate) fn page_count(&self) -> usize {
        self.page_count
    }

    pub(crate) fn max_pages(&self) -> usize {
        self.kind.page_count_max.unwrap_or(MAX_PAGES as u64) as usize
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

impl serde::Serialize for MemoryInstance {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use crate::PAGE_SIZE;
        use serde::ser::SerializeStruct;

        let mut state = serializer.serialize_struct("MemoryInstance", 4)?;
        state.serialize_field("kind", &self.kind)?;
        state.serialize_field("page_count", &self.page_count)?;
        state.serialize_field("ignored_page_region", &self.ignored_page_region)?;
        state.serialize_field("data_before_ignore", &self.data[..self.ignored_page_region.0 * PAGE_SIZE])?;
        state.serialize_field("data_after_ignore", &self.data[self.ignored_page_region.1 * PAGE_SIZE..])?;

        state.end()
    }
}

impl<'de> serde::Deserialize<'de> for MemoryInstance {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: serde::Deserializer<'de>,
    {
        use serde::de::{self, Deserialize, Deserializer, MapAccess, SeqAccess, Visitor};
        use std::fmt;

        enum Field {
            Kind,
            PageCount,
            IgnoredPageRegion,
            DataBeforeIgnore,
            DataAfterIgnore,
        }

        impl<'de> Deserialize<'de> for Field {
            fn deserialize<D>(deserializer: D) -> Result<Field, D::Error>
            where
                D: Deserializer<'de>,
            {
                struct FieldVisitor;

                impl<'de> Visitor<'de> for FieldVisitor {
                    type Value = Field;

                    fn expecting(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
                        formatter.write_str(
                            "`kind`, `page_count`, `ignored_page_region`, `data_before_ignore` or `data_after_ignore`",
                        )
                    }

                    fn visit_str<E>(self, value: &str) -> Result<Field, E>
                    where
                        E: de::Error,
                    {
                        match value {
                            "kind" => Ok(Field::Kind),
                            "page_count" => Ok(Field::PageCount),
                            "ignored_page_region" => Ok(Field::IgnoredPageRegion),
                            "data_before_ignore" => Ok(Field::DataBeforeIgnore),
                            "data_after_ignore" => Ok(Field::DataAfterIgnore),
                            _ => Err(de::Error::unknown_field(value, FIELDS)),
                        }
                    }
                }

                deserializer.deserialize_identifier(FieldVisitor)
            }
        }

        struct MemoryInstanceVisitor;

        impl<'de> Visitor<'de> for MemoryInstanceVisitor {
            type Value = MemoryInstance;

            fn expecting(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
                formatter.write_str("struct MemoryInstance")
            }

            fn visit_seq<V>(self, mut seq: V) -> Result<Self::Value, V::Error>
            where
                V: SeqAccess<'de>,
            {
                let kind = seq.next_element()?.ok_or_else(|| de::Error::invalid_length(0, &self))?;
                let page_count = seq.next_element()?.ok_or_else(|| de::Error::invalid_length(1, &self))?;
                let ignored_page_region: (usize, usize) =
                    seq.next_element()?.ok_or_else(|| de::Error::invalid_length(2, &self))?;

                // TODO: avoid allocation
                let data_before_ignore: Vec<u8> =
                    seq.next_element()?.ok_or_else(|| de::Error::invalid_length(3, &self))?;
                let data_after_ignore: Vec<u8> =
                    seq.next_element()?.ok_or_else(|| de::Error::invalid_length(4, &self))?;

                let mut data = vec![0; page_count * PAGE_SIZE];
                data[..ignored_page_region.0 * PAGE_SIZE].copy_from_slice(&data_before_ignore);
                data[ignored_page_region.1 * PAGE_SIZE..].copy_from_slice(&data_after_ignore);

                Ok(MemoryInstance { kind, page_count, ignored_page_region, data })
            }

            fn visit_map<V>(self, mut map: V) -> Result<Self::Value, V::Error>
            where
                V: MapAccess<'de>,
            {
                let mut kind = None;
                let mut page_count = None;
                let mut ignored_page_region: Option<(usize, usize)> = None;
                let mut data = None;
                while let Some(key) = map.next_key()? {
                    match key {
                        Field::Kind => {
                            if kind.is_some() {
                                return Err(de::Error::duplicate_field("kind"));
                            }
                            kind = Some(map.next_value()?);
                        }
                        Field::PageCount => {
                            if page_count.is_some() {
                                return Err(de::Error::duplicate_field("page_count"));
                            }
                            page_count = Some(map.next_value()?);
                            data = Some(vec![0u8; page_count.unwrap() * PAGE_SIZE]);
                        }
                        Field::IgnoredPageRegion => {
                            if ignored_page_region.is_some() {
                                return Err(de::Error::duplicate_field("ignored_page_region"));
                            }
                            ignored_page_region = Some(map.next_value()?);
                        }
                        Field::DataBeforeIgnore => {
                            let Some(data) = &mut data else {
                                return Err(de::Error::missing_field("page_count"));
                            };
                            let Some(ignored_page_region) = ignored_page_region else {
                                return Err(de::Error::missing_field("ignored_page_region"));
                            };
                            // TODO: avoid allocation
                            data[..ignored_page_region.0 * PAGE_SIZE].copy_from_slice(&map.next_value::<Vec<u8>>()?);
                        }
                        Field::DataAfterIgnore => {
                            let Some(data) = &mut data else {
                                return Err(de::Error::missing_field("page_count"));
                            };
                            let Some(ignored_page_region) = ignored_page_region else {
                                return Err(de::Error::missing_field("ignored_page_region"));
                            };
                            // TODO: avoid allocation
                            data[ignored_page_region.1 * PAGE_SIZE..].copy_from_slice(&map.next_value::<Vec<u8>>()?);
                        }
                    }
                }
                let kind = kind.ok_or_else(|| de::Error::missing_field("kind"))?;
                let page_count = page_count.ok_or_else(|| de::Error::missing_field("page_count"))?;
                let ignored_page_region: (usize, usize) =
                    ignored_page_region.ok_or_else(|| de::Error::missing_field("ignored_page_region"))?;
                let data = data.ok_or_else(|| de::Error::missing_field("page_count"))?;

                Ok(MemoryInstance { kind, page_count, ignored_page_region, data })
            }
        }

        const FIELDS: &[&str] =
            &["kind", "page_count", "ignored_page_region", "data_before_ignore", "data_after_ignore"];
        deserializer.deserialize_struct("MemoryInstance", FIELDS, MemoryInstanceVisitor)
    }
}
