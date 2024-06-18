#[deny(unsafe_op_in_unsafe_fn)]

pub mod reef {
    // Wasm imports
    #[link(wasm_import_module = "reef")]
    extern "C" {
        fn log(pointer: *const u8, length: i32);
        fn progress(percent: f32);
        fn sleep(seconds: f32);
        fn dataset_len() -> usize;
        fn dataset_write(pointer: i32);
    }

    /// Log a string to the Reef output
    pub fn reef_log(msg: &str) {
        unsafe { log(msg.as_ptr(), msg.len() as i32) }
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

    // SAFETY: only call once.
    pub unsafe fn _get_dataset() -> &'static [u8] {
        let len = unsafe { dataset_len() };
        let pages = len.div_ceil(PAGE_SIZE);

        let layout = std::alloc::Layout::from_size_align(pages * PAGE_SIZE, PAGE_SIZE).unwrap();
        let dataset_mem = unsafe { std::alloc::alloc(layout) };

        unsafe { dataset_write(dataset_mem as i32) };

        unsafe { std::slice::from_raw_parts(dataset_mem, len) }
    }

    pub mod prelude {
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

        #[macro_export]
        macro_rules! print {
            ($($arg:tt)*) => {
                panic!("'print!' not supported in Reef!");
            };
        }

        #[macro_export]
        macro_rules! dbg {
            ($($arg:tt)*) => {
                panic!("'dbg!' not supported in Reef!");
            };
        }
    }
}

mod input;

#[no_mangle]
pub extern "C" fn reef_main(_arg: i32) -> i32 {
    std::panic::set_hook(Box::new(|info| {
        reef::reef_log(&format!("PANIC: {}", info.to_string()));
    }));

    let dataset = unsafe { reef::_get_dataset() };
    input::run(&dataset);

    return 0;
}
