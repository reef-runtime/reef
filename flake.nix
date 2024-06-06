{
  description = "Environment for developing and deploying the Reef project.";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    rust-overlay.url = "github:oxalica/rust-overlay";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    rust-overlay,
    flake-utils,
    ...
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        overlays = [(import rust-overlay)];
        pkgs = import nixpkgs {
          inherit system overlays;
        };
        rustToolchain = pkgs.pkgsBuildHost.rust-bin.fromRustupToolchainFile ./rust-toolchain.toml;
        capnproto-go = pkgs.buildGoModule {
          # https://github.com/capnproto/go-capnp
          pname = "capnpc-go";
          version = "v3.0.0-alpha.31";

          src = pkgs.fetchFromGitHub {
            owner = "capnproto";
            repo = "go-capnp";
            rev = "ce7c84a071a503329dc21ed65cc3c99e7d74c9c7";
            hash = "sha256-gClpx4H2LoPpN0zuS6wLe+bEVpVa0/6B7JvL6USfEQM=";
          };

          subPackages = [
            "capnpc-go"
          ];
          doCheck = false;
          # modRoot = "./capnpc-go";

          vendorHash = "sha256-3RQBcJKjDacfINr/6jlzjGD7S/T0E223Wl6JPREFTlY=";

          meta = {
            description = "Cap'n Proto library and code generator for Go";
            homepage = "https://github.com/capnproto/go-capnp";
            license = pkgs.lib.licenses.mit;
            maintainers = [];
          };
        };
      in {
        devShells.default = pkgs.mkShell {
          name = "Reef Dev";

          buildInputs = with pkgs; [
            # Rust toolchain
            (rustToolchain.override {
              extensions = ["rust-src" "rust-std" "rust-analyzer"];
            })

            # Golang toolchain
            go_1_21
            richgo
            golangci-lint
            go-migrate

            # JS toolchain
            nodejs_20
            nodePackages.npm

            # Wasm tools
            wasmtime
            wabt
            binaryen
            llvmPackages_17.clang-unwrapped
            llvmPackages_17.bintools-unwrapped

            # Communications / Capnproto
            capnproto
            capnproto-rust
            capnproto-go

            # Misc
            ripgrep
            openssl
            pkg-config
            bruno
            perl
            findutils
            typos
            docker-compose
          ];

          shellHook = ''
            export OPENSSL_DIR="${pkgs.openssl.dev}"
            export OPENSSL_LIB_DIR="${pkgs.openssl.out}/lib"
            # export GOPATH=$(readlink -f ./go)
            # export PATH=$PATH:$GOPATH/bin

            # if running from zsh, reenter zsh
            if [[ $(ps -e | grep $PPID) == *"zsh" ]]; then
              zsh
              exit
            fi
          '';
        };

        formatter = nixpkgs.legacyPackages.${system}.alejandra;
      }
    );
}
