{
  description = "Environment for developing and deploying the Reef project.";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        flake-utils.follows = "flake-utils";
      };
    };
    crane = {
      url = "github:ipetkov/crane";
      inputs = {
        nixpkgs.follows = "nixpkgs";
      };
    };
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
    rust-overlay,
    crane,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        # ===================
        # General flake utils
        # ===================
        overlays = [(import rust-overlay)];
        pkgs = import nixpkgs {
          inherit system overlays;
        };
        inherit (pkgs) lib;

        # =========================
        # Go toolchain and packages
        # =========================

        # Custom packaging of teh Capnproto compiler for Go
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

          vendorHash = "sha256-3RQBcJKjDacfINr/6jlzjGD7S/T0E223Wl6JPREFTlY=";

          meta = {
            description = "Cap'n Proto library and code generator for Go";
            homepage = "https://github.com/capnproto/go-capnp";
            license = pkgs.lib.licenses.mit;
          };
        };

        # ===========================
        # Rust toolchain and packages
        # ===========================

        rustToolchain = (pkgs.pkgsBuildHost.rust-bin.fromRustupToolchainFile ./rust-toolchain.toml).override {
          extensions = ["rust-src" "rust-std" "rust-analyzer"];
          targets = ["wasm32-unknown-unknown"];
        };
        # Configure crane
        craneLib = (crane.mkLib pkgs).overrideToolchain rustToolchain;
        # cf. https://crane.dev/API.html#libcleancargosource

        # Note: this triggers rebuilds when any files are changed, but none Rust files can't be ignored
        # because *.capnp files need to be included (otherwise craneLib.cleanCargoSource ./.;)
        src = craneLib.cleanCargoSource (craneLib.path ./.);

        # Build tools
        nativeBuildInputs = with pkgs; [
          pkg-config
          git

          rustToolchain

          capnproto
          capnproto-rust
          capnproto-go
        ];
        # buildInputs = with pkgs; [openssl sqlite];
        buildInputs = [];
        commonArgs = {
          inherit src buildInputs nativeBuildInputs;

          strictDeps = true;

          # Additional environment variables can be set directly
          # MY_CUSTOM_VAR = "some value";
        };
        cargoArtifacts = craneLib.buildDepsOnly (commonArgs // {pname = "reef_dependencies";});
        # cargoArtifacts = craneLib.buildDepsOnly commonArgs;

        individualCrateArgs =
          commonArgs
          // {
            inherit cargoArtifacts;
            inherit (craneLib.crateNameFromCargoToml {inherit src;}) version;
          };

        fileSetForCrate = crate:
          lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions ([
                # Top level Cargo files
                ./Cargo.toml
                ./Cargo.lock

                # All other Cargo.toml need to be included for the top level one to parse
                ./reef_compiler/Cargo.toml
                ./reef_interpreter/Cargo.toml
                ./reef_node_native/Cargo.toml
                ./reef_protocol/compiler/rust/Cargo.toml
                ./reef_protocol/node/rust/Cargo.toml
              ]
              ++ crate);
          };

        # Build the top-level crates of the workspace as individual derivations.
        reef_node_native = craneLib.buildPackage (individualCrateArgs
          // {
            pname = "reef_node_native";
            cargoExtraArgs = "-p reef_node_native";
            src = fileSetForCrate [./reef_node_native ./reef_interpreter ./reef_protocol];
          });
        reef_compiler = craneLib.buildPackage (individualCrateArgs
          // {
            # TODO: this is missing all the compiler tools
            pname = "reef_compiler";
            cargoExtraArgs = "-p reef_compiler";
            src = fileSetForCrate [./reef_compiler ./reef_protocol];
          });
      in {
        devShells.default = pkgs.mkShell {
          name = "Reef Dev";

          buildInputs = with pkgs; [
            # Rust toolchain
            rustToolchain

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
              export SHELL=zsh
              zsh
              exit
            fi
          '';
        };

        formatter = nixpkgs.legacyPackages.${system}.alejandra;

        packages = {
          inherit reef_node_native reef_compiler;
        };

        apps = {
          reef-node-native = flake-utils.lib.mkApp {
            drv = reef_node_native;
          };
          reef-compiler = flake-utils.lib.mkApp {
            drv = reef_compiler;
          };
        };
      }
    );
}
