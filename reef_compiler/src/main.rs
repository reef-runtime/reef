use std::{
    net::{IpAddr, SocketAddr, ToSocketAddrs},
    path::PathBuf,
    str::FromStr,
};

use capnp_rpc::{rpc_twoparty_capnp, twoparty, RpcSystem};
use clap::{Parser, Subcommand};

use anyhow::{bail, Context, Result};
use compiler_capnp::compiler;
use futures::AsyncReadExt;
use reef_protocol_compiler::compiler_capnp::{self, compiler_response};
use server::{run_server_main, Compiler};

mod server;

#[derive(Subcommand, PartialEq, Eq, Debug)]
pub enum Command {
    /// Spawn a compiler server.
    Server {
        /// The port on which the server listens.
        port: u16,
        /// Skeleton path.
        skeleton_path: PathBuf,
        /// Compilation / build working dir.
        build_path: PathBuf,

        #[arg(short = 'c', long)]
        // Whether to skip cleanup after compilation.
        no_cleanup: bool,
    },
    /// Connect to an existing server.
    Client {
        /// The ip of the remote compilation server.
        ip: String,
        /// The port of the remote compilation server.
        port: u16,
    },
}

/// Reef compiler service.
#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    #[clap(subcommand)]
    pub subcommand: Command,
}

#[tokio::main(flavor = "current_thread")]
pub async fn main() -> Result<()> {
    let args = Args::parse();

    match args.subcommand {
        Command::Server {
            port,
            skeleton_path,
            build_path,
            no_cleanup,
        } => {
            let addr = SocketAddr::new(
                IpAddr::from_str("0.0.0.0").with_context(|| "IP is always valid")?,
                port,
            );

            run_server_main(
                addr,
                Compiler {
                    build_path,
                    skeleton_path,
                    no_cleanup,
                },
            )
            .await
            .with_context(|| "failed to run server")
        }
        Command::Client { ip, port } => {
            let addr = format!("{ip}:{port}")
                .to_socket_addrs()?
                .next()
                .with_context(|| "server url is invalid")?;

            tokio::task::LocalSet::new()
                .run_until(async move {
                    let stream = tokio::net::TcpStream::connect(&addr)
                        .await
                        .with_context(|| "could not connect to compiler server")?;
                    stream.set_nodelay(true)?;
                    let (reader, writer) =
                        tokio_util::compat::TokioAsyncReadCompatExt::compat(stream).split();
                    let rpc_network = Box::new(twoparty::VatNetwork::new(
                        reader,
                        writer,
                        rpc_twoparty_capnp::Side::Client,
                        Default::default(),
                    ));
                    let mut rpc_system = RpcSystem::new(rpc_network, None);
                    let compiler: compiler::Client =
                        rpc_system.bootstrap(rpc_twoparty_capnp::Side::Server);

                    tokio::task::spawn_local(rpc_system);

                    let mut request = compiler.compile_request();
                    request.get().set_program_src("int main() {}");
                    request.get().set_language(compiler_capnp::Language::C);

                    let reply = request
                        .send()
                        .promise
                        .await
                        .with_context(|| "could not send compilation request")?;

                    let response = reply
                        .get()?
                        .get_response()
                        .with_context(|| "failed to receive compilation response")?;

                    // TODO: what the ACTUAL FUCK?
                    match response.which() {
                        Ok(compiler_response::FileContent(Ok(e))) => {
                            println!("ok, received file: {e:?}")
                        }
                        Ok(compiler_response::FileContent(Err(e))) => {
                            bail!("error receiving file: {e}")
                        }

                        Ok(compiler_response::CompilerError(Ok(err))) => {
                            bail!("compile error: {}", err.to_str().unwrap())
                        }
                        Ok(compiler_response::CompilerError(Err(e))) => bail!("compile error: {e}"),

                        Ok(compiler_response::SystemError(Ok(e))) => {
                            bail!("FS error: {e:?}")
                        }
                        Ok(compiler_response::SystemError(Err(e))) => println!("fs error: {e}"),

                        Err(e) => bail!("general error: {e}"),
                    }

                    Ok(())
                })
                .await
        }
    }
}
