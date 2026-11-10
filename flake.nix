{
  description = "Subsoil application exchange over an independent OCI registry";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      eachSystem = nixpkgs.lib.genAttrs systems;
      packageFor = system:
        let pkgs = import nixpkgs { inherit system; };
        in pkgs.rustPlatform.buildRustPackage {
          pname = "subsoil";
          version = "0.2.0";
          src = pkgs.lib.cleanSourceWith {
            src = ./.;
            filter = path: type:
              let
                relative = pkgs.lib.removePrefix ((toString ./.) + "/") (toString path);
              in (toString path) == (toString ./.) || builtins.elem relative [ "Cargo.toml" "Cargo.lock" "LICENSE" "src" "tests" "assets" "assets/brand" ]
                 || pkgs.lib.hasPrefix "src/" relative || pkgs.lib.hasPrefix "tests/" relative
                 || pkgs.lib.hasPrefix "assets/brand/" relative;
          };
          cargoLock = {
            lockFile = ./Cargo.lock;
            outputHashes."ifc-lite-core-25.0.0" = "sha256-TtBp7dY9Nu4zsVUl3ZmH5BAIF4+xF6ZsP6n81/oMJHQ=";
          };
          nativeBuildInputs = [ pkgs.pkg-config ];
          # Networked integration tests run explicitly through the demo app.
          cargoTestFlags = [ "--lib" ];
          meta.mainProgram = "subsoil";
          meta.license = pkgs.lib.licenses.asl20;
        };
    in {
      packages = eachSystem (system: { default = packageFor system; });
      apps = eachSystem (system:
        let
          pkgs = import nixpkgs { inherit system; };
          package = packageFor system;
        in {
          default = { type = "app"; program = "${package}/bin/subsoil"; meta.description = "Application handoffs and OCI exchange tools"; };
        });
      devShells = eachSystem (system:
        let pkgs = import nixpkgs { inherit system; };
        in { default = pkgs.mkShell {
          packages = with pkgs; [ cargo rustc rustfmt clippy pkg-config oras jq nodejs ];
        }; });
      checks = eachSystem (system: { build = packageFor system; });
    };
}
