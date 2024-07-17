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

          vendorHash = "sha256-oBMy9XmST4id+z2RhzD8BsC6KyTJpauN07n3rWv34iw=";
          # vendorHash = lib.fakeHash;

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
        };
        nativeArgs = {
          CARGO_BUILD_TARGET = "x86_64-unknown-linux-musl";
          CARGO_BUILD_RUSTFLAGS = "-C target-feature=+crt-static";
        };
        wasmArgs = {
          CARGO_BUILD_TARGET = "wasm32-unknown-unknown";
        };
        cargoArtifacts = craneLib.buildDepsOnly (commonArgs
          // {
            src = lib.fileset.toSource {
              root = ./.;
              fileset = ./Cargo.lock;
            };
            pname = "reef_dependencies";
            version = "0.0.1";
          });

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
                ./reef_node_web/Cargo.toml
                ./reef_protocol/compiler/rust/Cargo.toml
                ./reef_protocol/node/rust/Cargo.toml
                ./reef_protocol/reef_wasm_interface/Cargo.toml
              ]
              ++ crate);
          };

        # Build the top-level crates of the workspace as individual derivations.
        reef_compiler_bin = craneLib.buildPackage (individualCrateArgs
          // nativeArgs
          // {
            pname = "reef_compiler";
            cargoExtraArgs = "-p reef_compiler";
            src = fileSetForCrate [./reef_compiler/src ./reef_protocol];
          });
        reef_node_native = craneLib.buildPackage (individualCrateArgs
          // nativeArgs
          // {
            pname = "reef_node_native";
            cargoExtraArgs = "-p reef_node_native";
            src = fileSetForCrate [./reef_node_native ./reef_interpreter ./reef_protocol];
          });
        reef_node_web_bin = craneLib.buildPackage (individualCrateArgs
          // wasmArgs
          // {
            pname = "reef_node_web";
            cargoExtraArgs = "-p reef_node_web";
            src = fileSetForCrate [./reef_node_web ./reef_interpreter ./reef_protocol];
            doCheck = false;
          });

        # =============
        # Reef Compiler
        # =============

        compilerRustToolchain =
          pkgs.pkgsBuildHost.rust-bin.stable.latest.minimal.override
          {
            targets = ["wasm32-unknown-unknown"];
          };
        compilerCraneLib = (crane.mkLib pkgs).overrideToolchain compilerRustToolchain;
        compilerCargoArtifacts = craneLib.buildDepsOnly {
          nativeBuildInputs = [
            compilerRustToolchain
          ];

          strictDeps = true;

          src = ./reef_compiler/lang_templates/rust;
          pname = "reef_compiler_placeholder";
          version = "0.1.0";

          CARGO_BUILD_TARGET = "wasm32-unknown-unknown";
        };

        reefCompilerToolchain = with pkgs; [
          bashInteractive

          gnumake
          coreutils
          gnused
          gcc
          binaryen

          llvmPackages_18.clang-unwrapped
          llvmPackages_18.bintools-unwrapped
          compilerRustToolchain
        ];

        langTemplates =
          pkgs.stdenv.mkDerivation
          {
            name = "reef_lang_templates";
            src = ./reef_compiler/lang_templates;
            buildInputs = [pkgs.zstd];
            buildPhase = ''
              mkdir rust/target
              cd rust/target
              cp ${compilerCargoArtifacts}/target.tar.zst .
              tar --use-compress-program=unzstd -xvf target.tar.zst
              rm target.tar.zst
              cd ../..
            '';
            installPhase = ''
              mkdir -p $out
              cp -R . $out/
            '';
          };

        # Wrap reef_compiler_bin in shell script to correctly include lang templates
        # and runtime compiler tools that are required.
        reef_compiler = pkgs.writeShellApplication {
          name = "reef_compiler";

          runtimeInputs = reefCompilerToolchain ++ [reef_compiler_bin];

          text = ''
            ${reef_compiler_bin}/bin/reef_compiler --lang-templates ${langTemplates} "$@"
          '';
        };

        # ===============
        # JS/Npm Packages
        # ===============

        reef_frontend = pkgs.buildNpmPackage {
          pname = "reef_frontend";
          version = "0.1.0";

          src = lib.fileset.toSource {
            root = ./.;
            fileset = ./reef_frontend;
          };

          npmDepsHash = "sha256-p5blKotnweTz0qYgZdXO2DSqVRHBUAMkm4e4ED3PTfI=";
          # npmDepsHash = lib.fakeHash;

          npmPackFlags = ["--ignore-scripts"];

          postPatch = ''
            cd reef_frontend
            cp -r ${reef_node_web}/pkg/ ./src/lib/node_web_generated
            cp -r ${reef_node_native}/bin/reef_node_native ./public
          '';

          installPhase = ''
            cp -r out $out
          '';
        };

        reef_node_web = pkgs.stdenv.mkDerivation {
          name = "reef_node_web";
          src = lib.fileset.toSource {
            root = ./.;
            fileset = lib.fileset.unions [];
          };
          nativeBuildInputs = with pkgs; [
            wasm-bindgen-cli
            binaryen
          ];

          unpackPhase = ''
            cp -r ${reef_node_web_bin}/lib .
          '';
          buildPhase = ''
            echo "Running wasm-bindgen..."
            wasm-bindgen --out-dir pkg --target web ./lib/reef_node_web.wasm
            echo "Running wasm-opt..."
            wasm-opt -o ./pkg/reef_node_web_bg.wasm -O4 --strip-debug ./pkg/reef_node_web_bg.wasm
          '';
          installPhase = ''
            mkdir -p $out
            cp -r pkg $out
          '';
        };

        # ================
        # Container images
        # ================

        container_tmp =
          # Creating /tmp in the container.
          pkgs.stdenv.mkDerivation
          {
            name = "container_tmp";
            src = ./reef_compiler;
            buildPhase = " ";
            installPhase = "mkdir -p $out/tmp";
          };

        reef_caddy_image = pkgs.dockerTools.streamLayeredImage {
          name = "reef_caddy";
          tag = "latest";

          contents = [
            (pkgs.stdenv.mkDerivation
              {
                name = "reef_caddy_content";
                src = lib.fileset.toSource {
                  root = ./.;
                  fileset = ./Caddyfile;
                };
                buildPhase = " ";
                installPhase = ''
                  mkdir -p $out/static
                  cp -r ${reef_frontend}/. $out/static
                  cp ./Caddyfile $out/Caddyfile
                '';
              })
            pkgs.caddy
          ];
          config = {
            Cmd = ["/bin/caddy" "run" "--config" "/Caddyfile"];
          };
        };

        jobTemplates =
          pkgs.stdenv.mkDerivation
          {
            name = "reef_job_templates";
            src = ./reef_manager/example_job_templates;
            buildPhase = " ";
            installPhase = ''
              mkdir -p $out/job_templates
              cp -R . $out/job_templates
            '';
          };
        reef_manager_image = pkgs.dockerTools.streamLayeredImage {
          name = "reef_manager";
          tag = "latest";
          contents = [reef_manager jobTemplates container_tmp];
          config = {
            Cmd = ["bin/reef_manager"];
          };
        };

        reef_compiler_image = pkgs.dockerTools.streamLayeredImage {
          name = "reef_compiler";
          tag = "latest";

          contents = [reef_compiler container_tmp pkgs.cacert];
          config = {
            Cmd = ["bin/reef_compiler" "--build-path" "/tmp"];
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
            wasm-bindgen-cli

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
          # Binary outputs.
          inherit reef_manager reef_compiler reef_node_native;
          # Other outputs.
          inherit reef_frontend reef_node_web_bin reef_node_web;

          # Container images for central system.
          inherit reef_caddy_image reef_manager_image reef_compiler_image;
          # Container images for node.
          inherit reef_node_native_image;

          inherit langTemplates compilerCargoArtifacts;
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
