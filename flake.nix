{
  description = "Environment for developing and deploying the Reef project.";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
    rust-overlay = {
      url = "github:oxalica/rust-overlay";
      inputs = {
        nixpkgs.follows = "nixpkgs";
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
          # config.allowUnfree = true;
        };
        inherit (pkgs) lib;

        # =========================
        # Go toolchain and packages
        # =========================

        # Custom packaging of the Capnproto compiler for Go
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

        reef_manager = pkgs.buildGoModule {
          pname = "reef_manager";
          version = "v0.0.1";

          src = lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions [
              ./reef_manager
              ./reef_protocol
            ];
          };
          modRoot = "./reef_manager";
          doCheck = false;

          vendorHash = "sha256-nBL00njherjwkakvgqPR4kLSZnUseMVMP0lqjaXPB2g=";

          meta = {
            description = "Central management server for Reef distributed compute system";
            homepage = "https://github.com/reef-runtime/reef";
          };
        };

        # ===========================
        # Rust toolchain and packages
        # ===========================

        rustToolchain = (pkgs.pkgsBuildHost.rust-bin.fromRustupToolchainFile ./rust-toolchain.toml).override {
          extensions = ["rust-src" "rust-std" "rust-analyzer"];
          targets = ["wasm32-unknown-unknown" "x86_64-unknown-linux-musl"];
        };
        # Configure crane
        craneLib = (crane.mkLib pkgs).overrideToolchain rustToolchain;
        # cf. https://crane.dev/API.html#libcleancargosource

        # Note: this triggers rebuilds when any files are changed, but none Rust files can't be ignored
        # because *.capnp files need to be included (otherwise craneLib.cleanCargoSource ./.;)
        src = craneLib.cleanCargoSource (craneLib.path ./.);

        # Build tools
        nativeBuildInputs = with pkgs; [
          git

          rustToolchain

          capnproto
          capnproto-rust
          capnproto-go
        ];
        buildInputs = with pkgs; [];
        commonArgs = {
          inherit src buildInputs nativeBuildInputs;

          strictDeps = true;

          CARGO_BUILD_TARGET = "x86_64-unknown-linux-musl";
          CARGO_BUILD_RUSTFLAGS = "-C target-feature=+crt-static";
        };
        cargoArtifacts = craneLib.buildDepsOnly (commonArgs // {pname = "reef_dependencies";});

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
        reef_compiler_bin = craneLib.buildPackage (individualCrateArgs
          // {
            pname = "reef_compiler";
            cargoExtraArgs = "-p reef_compiler";
            src = fileSetForCrate [./reef_compiler/src ./reef_protocol];
          });
        reef_node_native = craneLib.buildPackage (individualCrateArgs
          // {
            pname = "reef_node_native";
            cargoExtraArgs = "-p reef_node_native";
            src = fileSetForCrate [./reef_node_native ./reef_interpreter ./reef_protocol];
          });

        # ================
        # Conatiner images
        # ================

        reef_manager_image = pkgs.dockerTools.streamLayeredImage {
          name = "reef_manager";
          tag = "latest";
          contents = [reef_manager];
          config = {
            Cmd = ["bin/reef_manager"];
          };
        };

        # Wrap reef_compiler_bin in shell script to correctly include lang templates
        # and runtime compiler tools that are required.
        reef_compiler = pkgs.writeShellApplication {
          name = "reef_compiler";

          runtimeInputs = with pkgs; [
            bashInteractive

            gnumake
            coreutils
            gnused
            llvmPackages_18.clang-unwrapped
            llvmPackages_18.bintools-unwrapped
            (pkgs.pkgsBuildHost.rust-bin.stable.latest.minimal.override
              {
                targets = ["wasm32-unknown-unknown"];
              })
            binaryen

            reef_compiler_bin
          ];

          text = ''
            ${reef_compiler_bin}/bin/reef_compiler --lang-templates ${./reef_compiler/lang_templates} "$@"
          '';
        };
        reef_compiler_image = pkgs.dockerTools.streamLayeredImage {
          name = "reef_compiler";
          tag = "latest";

          contents = [reef_compiler ./reef_compiler/container_tmp];
          config = {
            Cmd = ["bin/reef_compiler"];
          };
        };

        reef_node_native_image = pkgs.dockerTools.streamLayeredImage {
          name = "reef_node_native";
          tag = "latest";
          contents = [reef_node_native];
          config = {
            Cmd = ["bin/reef_node_native"];
          };
        };
      in {
        # =========
        # Dev Shell
        # =========

        devShells.default = pkgs.mkShell {
          name = "Reef Dev";

          buildInputs = with pkgs; [
            # Build tools
            llvmPackages_18.clang-unwrapped
            llvmPackages_18.bintools-unwrapped
            gnumake

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
            nodePackages.pnpm

            # Wasm tools
            wasmtime
            wabt
            binaryen

            # Communications / Capnproto
            capnproto
            capnproto-rust
            capnproto-go

            # Containers
            docker-compose
            dive

            # Misc
            ripgrep
            openssl
            pkg-config
            bruno
            perl
            findutils
            typos
            caddy
          ];

          shellHook = ''
            export OPENSSL_DIR="${pkgs.openssl.dev}"
            export OPENSSL_LIB_DIR="${pkgs.openssl.out}/lib"

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
          # Raw binary outputs
          inherit reef_manager reef_compiler reef_node_native;

          # Conatiner images
          inherit reef_manager_image reef_compiler_image reef_node_native_image;
        };

        apps = {
          reef-manager = flake-utils.lib.mkApp {
            drv = reef_manager;
          };
          reef-compiler = flake-utils.lib.mkApp {
            drv = reef_compiler;
          };
          reef-node-native = flake-utils.lib.mkApp {
            drv = reef_node_native;
          };
        };
      }
    );
}
