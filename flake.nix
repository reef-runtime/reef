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
      in {
        devShells.default = pkgs.mkShell {
          name = "Reef Dev";

          buildInputs = with pkgs; [
            (rustToolchain.override {
              extensions = ["rust-src" "rust-std" "rust-analyzer"];
            })

            # Golang.
            go_1_21
            richgo
            golangci-lint
            go-migrate

            # Node JS.
            nodejs_20
            nodePackages.npm

            # Misc.
            ripgrep
          ];

          shellHook = ''
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
