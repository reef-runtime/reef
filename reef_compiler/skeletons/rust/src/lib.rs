use reef::{ReefResult, _set_result};

#[deny(unsafe_op_in_unsafe_fn)]

pub mod reef {
    // Wasm imports
    #[link(wasm_import_module = "reef")]
    extern "C" {
        fn log(ptr: *const u8, len: usize);
        fn progress(done: f32);
        fn sleep(seconds: f32);
        fn dataset_len() -> usize;
        fn dataset_write(ptr: *mut u8);
        fn result(result_type: i32, ptr: *const u8, len: usize);
    }

    /// Log a string to the Reef output
    pub fn reef_log(msg: &str) {
        unsafe { log(msg.as_ptr(), msg.len()) }
    }

    /// Report progress of this operation to the user
    pub fn reef_progress(done: f32) {
        //? Should the range be checked here?
        unsafe { progress(done) }
    }

    /// Sleep for a specified amount of seconds
    pub fn reef_sleep(seconds: f32) {
        unsafe { sleep(seconds) }
    }

    const PAGE_SIZE: usize = 65536;

    /// SAFETY: only call once.
    #[doc(hidden)]
    pub unsafe fn _get_dataset() -> &'static [u8] {
        let len = unsafe { dataset_len() };
        let pages = len.div_ceil(PAGE_SIZE);

        let layout = std::alloc::Layout::from_size_align(pages * PAGE_SIZE, PAGE_SIZE).unwrap();
        let dataset_mem = unsafe { std::alloc::alloc(layout) };

        unsafe { dataset_write(dataset_mem) };

        unsafe { std::slice::from_raw_parts(dataset_mem, len) }
    }

    #[doc(hidden)]
    pub unsafe fn _set_result(result_type: i32, data: &[u8]) {
        unsafe { result(result_type, data.as_ptr(), data.len()) }
    }

    pub struct ReefResult {
        pub content_type: i32,
        pub data: Vec<u8>,
    }

    impl From<()> for ReefResult {
        fn from(_value: ()) -> Self {
            ReefResult { content_type: 0, data: 0i64.to_le_bytes().to_vec() }
        }
    }
    macro_rules! impl_result_from_int {
        ($int: ty) => {
            impl From<$int> for ReefResult {
                fn from(value: $int) -> Self {
                    ReefResult { content_type: 0, data: value.to_le_bytes().to_vec() }
                }
            }
        };
    }
    impl_result_from_int!(isize);
    impl_result_from_int!(usize);
    impl_result_from_int!(i32);
    impl_result_from_int!(u32);
    impl_result_from_int!(i64);
    impl_result_from_int!(u64);

    impl From<Vec<u8>> for ReefResult {
        fn from(value: Vec<u8>) -> Self {
            ReefResult { content_type: 1, data: value }
        }
    }
    impl From<String> for ReefResult {
        fn from(value: String) -> Self {
            ReefResult { content_type: 2, data: value.into_bytes() }
        }
    }

    pub mod prelude {
        // Reef
        pub use super::{reef_log, reef_progress, reef_sleep, ReefResult};

        // Dynamic borrow checking
        pub use std::cell::{Cell, RefCell};
        pub use std::rc::{Rc, Weak};

        // Growable Array collections (vector)
        pub use std::collections::VecDeque;

        // Hash collections (via BTreeMap)
        pub use std::collections::{BTreeMap, BTreeSet, HashMap, HashSet};
        // Other collections
        pub use std::collections::{BinaryHeap, LinkedList};

        #[macro_export]
        macro_rules! println {
            () => {
                $crate::reef::reef_log!("");
            };
            ($($arg:tt)*) => {{
                $crate::reef::reef_log(&format!($($arg)*));
            }};
        }
        pub use println;

        #[macro_export]
        macro_rules! print {
            ($($arg:tt)*) => {
                panic!("'print!' not supported in Reef!");
            };
        }
        pub use print;

        #[macro_export]
        macro_rules! dbg {
            ($($arg:tt)*) => {
                panic!("'dbg!' not supported in Reef!");
            };
        }
        pub use dbg;
    }
}

mod input;

#[no_mangle]
pub extern "C" fn reef_main() {
    // setup in-Wasm environment
    std::panic::set_hook(Box::new(|info| {
        reef::reef_log(&format!("PANIC: {}", info.to_string()));
    }));
    let dataset = unsafe { reef::_get_dataset() };

    // Run user code
    let res: ReefResult = input::run(&dataset).into();

    // Return result
    unsafe { _set_result(res.content_type, &res.data) }
}
