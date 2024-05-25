use std::io::ErrorKind;
use std::str::FromStr;

use argh::FromArgs;
// use args::WasmArg;
use color_eyre::eyre::Result;
use log::{debug, info};
use tinywasm::types::WasmValue;
use tinywasm::{Extern, FuncContext, MemoryStringExt};
use tinywasm::{Imports, Module};

// use crate::args::to_wasm_args;
// mod args;
// mod util;

#[cfg(feature = "wat")]
mod wat;

#[derive(FromArgs)]
/// TinyWasm CLI
struct TinyWasmCli {
    #[argh(subcommand)]
    nested: TinyWasmSubcommand,

    /// log level
    #[argh(option, short = 'l', default = "\"info\".to_string()")]
    log_level: String,
}

#[derive(FromArgs)]
#[argh(subcommand)]
enum TinyWasmSubcommand {
    Run(Run),
}

enum Engine {
    Main,
}

impl FromStr for Engine {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "main" => Ok(Self::Main),
            _ => Err(format!("unknown engine: {}", s)),
        }
    }
}

#[derive(FromArgs)]
/// run a wasm file
#[argh(subcommand, name = "run")]
struct Run {
    /// wasm file to run
    #[argh(positional)]
    wasm_file: String,
    // /// engine to use
    // #[argh(option, short = 'e', default = "Engine::Main")]
    // engine: Engine,
}

fn main() -> Result<()> {
    color_eyre::install()?;

    let args: TinyWasmCli = argh::from_env();

    let cwd = std::env::current_dir()?;

    match args.nested {
        TinyWasmSubcommand::Run(Run { wasm_file }) => {
            let path = cwd.join(wasm_file.clone());
            let module = match wasm_file.ends_with(".wat") {
                true => {
                    return Err(color_eyre::eyre::eyre!(
                        "wat support is not enabled in this build"
                    ))
                }
                false => tinywasm::Module::parse_file(path)?,
            };

            run(module)
        }
    }
}

fn run(module: Module) -> Result<()> {
    let mut store = tinywasm::Store::default();

    let mut imports = Imports::new();

    // function args can be either a single
    // value that implements `TryFrom<WasmValue>` or a tuple of them
    // let print_i32 = Extern::typed_func(|_ctx: tinywasm::FuncContext<'_>, arg: i32| {
    //     log::debug!("print_i32: {}", arg);
    //     Ok(())
    // });

    // let log = Extern::typed_func(|_ctx: tinywasm::FuncContext<'_>, str_pointer: i32| {
    //     let pointer = str_pointer as *const c_char;
    //     let c_str: &CStr = unsafe { CStr::from_ptr(pointer) };
    //     let log_content = c_str.to_str().unwrap();
    //
    //     println!("Log content: {log_content}");
    //
    //     println!("PRINT GREETING!!!");
    //     // log::debug!("PRINT ");
    //     Ok(())
    // });

    imports.define(
        "reef",
        "reef_log",
        Extern::typed_func(|mut ctx: FuncContext<'_>, args: (i32, i32)| {
            let mem = ctx.exported_memory("memory")?;
            let ptr = args.0 as usize;
            let len = args.1 as usize;
            let string = mem.load_string(ptr, len)?;
            println!("REEF_LOG: {}", string);
            Ok(())
        }),
    )?;

    imports.define(
        "reef",
        "report_progress",
        Extern::typed_func(|mut _ctx: FuncContext<'_>, percent: i32| {
            if percent < 0 || percent > 100 {
                return Err(tinywasm::Error::Io(std::io::Error::new(
                    ErrorKind::AddrNotAvailable,
                    "Invalid range: percentage must be in 0..=100",
                )));
            }

            println!("REEF_REPORT_PROGRESS: {percent}");
            Ok(())
        }),
    )?;

    // let table_type = TableType::new(ValType::RefFunc, 10, Some(20));
    // let table_init = WasmValue::default_for(ValType::RefFunc);

    imports
        // .define("reef", "log", log)?
        // .define("my_module", "table", Extern::table(table_type, table_init))?
        // .define("my_module", "memory", Extern::memory(MemoryType::new_32(1, Some(2))))?
        // .define("my_module", "global_i32", Extern::global(WasmValue::I32(666), false))?
        .link_module("my_other_module", 0)?;

    let instance = module.instantiate(&mut store, Some(imports))?;

    // if let Some(func) = func {
    //     let func = instance.exported_func_untyped(&store, &func)?;
    //     let res = func.call(&mut store, &args)?;
    //     info!("{res:?}");
    // }

    Ok(())
}
