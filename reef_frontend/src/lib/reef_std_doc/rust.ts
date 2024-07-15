import { DocLang, DocSection } from '.';

const mainSection: DocSection = {
  name: 'User main function',
  entries: [
    {
      signature: 'pub fn run(dataset: &[u8]) -> impl Into<ReefOutput> {}',
      description: [
        'The entry function for a job written by Reef users. This function must be provided in each job submission because it will be called by wrapper code during execution on a Reef Node.',
        'As an argument you are passed s slice (pointer) to the dataset which you can safely read from.',
        'You can set the job output by returning any datastructure which can be converted to a supported output type. See `ReefOutput` for more information.',
      ],
    },
  ],
};

const reefSection: DocSection = {
  name: 'Reef Functions',
  description: ['Functions for interacting with the Reef system from jobs.'],
  entries: [
    {
      signature: 'pub fn reef_progress(done: f32) {}',
      description: [
        'Reports the current process to the system.',
        'As an argument you have to pass the progress as a f32 value between 0 and 1.',
      ],
    },
    {
      signature: 'pub fn reef_sleep(seconds: f32) {}',
      description: [
        'Sleeps for the given duration.',
        'As an argument you have to pass the time in seconds as a f32.',
      ],
    },
    {
      signature: 'pub fn reef_log(msg: &str) {}',
      description: [
        'Logs the given string.',
        'As an argument you have to pass the message as a str.',
      ],
    },
    {
      signature: 'macro_rules! println { ($($arg:tt)*) => { ... }; }',
      description: ['Macro for logging the given format string.'],
      example: 'println!("format {} arguments", "some");',
    },
  ],
};

const librarySection: DocSection = {
  name: 'Library Functions',
  description: [
    "Thanks to Rust's great support for Wasm almost the entire Rust standard library is available to Reef jobs.",
    'This includes many common data structures like arrays, vectors (`Vec`) and hashmaps.',
    'The only mayor limitation is that all APIs related to interfacing with the operating system, like files and networking, are not available.',
  ],
  entries: [],
};

export const rustStdDoc: DocLang = {
  description: [
    'This is Documentation for the Reef Standard Library for Rust. It mainly documents all the functions exposed to user submitted code.',
    'All items listed here are automatically in scope thanks to a hidden prelude import.',
  ],
  sections: [mainSection, reefSection, librarySection],
};
