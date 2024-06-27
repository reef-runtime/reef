use std::{
    net::{IpAddr, SocketAddr},
    path::PathBuf,
    str::FromStr,
};

use capnp_rpc::{rpc_twoparty_capnp, twoparty, RpcSystem};
use futures::AsyncReadExt;

use anyhow::{Context, Result};
use clap::Parser;

use reef_protocol_compiler::compiler_capnp::compiler;

mod server;

/// Reef compiler service.
#[derive(Parser, Debug)]
#[command(version, about, long_about = None)]
struct Args {
    /// The port on which the server listens.
    port: u16,
    /// Compilation / build working dir.
    build_path: PathBuf,

    #[arg(short = 't', long)]
    /// Path to language templates.
    lang_templates: Option<PathBuf>,
    #[arg(short = 'c', long)]
    // Whether to skip cleanup after compilation.
    no_cleanup: bool,
}

#[tokio::main(flavor = "current_thread")]
pub async fn main() -> Result<()> {
    let args = Args::parse();

    let addr = SocketAddr::new(IpAddr::from_str("0.0.0.0")?, args.port);

    println!("Reef compiler RPC running at {addr}");

    let compiler = server::Compiler {
        build_path: args.build_path,
        lang_templates: args.lang_templates.unwrap_or(PathBuf::from_str("./lang_templates").unwrap()),
        no_cleanup: args.no_cleanup,
    };

    tokio::task::LocalSet::new()
        .run_until(async move {
            let listener = tokio::net::TcpListener::bind(&addr).await.with_context(|| "failed to bind to socket")?;
            let rpc_client: compiler::Client = capnp_rpc::new_client(compiler);

            loop {
                let (stream, _) = listener.accept().await.with_context(|| "failed to listen")?;
                stream.set_nodelay(true)?;
                let (reader, writer) = tokio_util::compat::TokioAsyncReadCompatExt::compat(stream).split();
                let network =
                    twoparty::VatNetwork::new(reader, writer, rpc_twoparty_capnp::Side::Server, Default::default());

                let rpc_system = RpcSystem::new(Box::new(network), Some(rpc_client.clone().client));
                tokio::task::spawn_local(rpc_system);
            }
        })
        .await
}
