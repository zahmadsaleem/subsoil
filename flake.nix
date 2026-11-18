{
  description = "Subsoil application exchange over an independent OCI registry";
  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  outputs = { self, nixpkgs }:
    let
      systems = [ "aarch64-darwin" "x86_64-darwin" "aarch64-linux" "x86_64-linux" ];
      eachSystem = nixpkgs.lib.genAttrs systems;
      packageFor = system:
        let pkgs = import nixpkgs { inherit system; };
        in pkgs.buildGoModule {
          pname = "subsoil";
          version = "0.2.0";
          src = pkgs.lib.fileset.toSource {
            root = ./.;
            fileset = pkgs.lib.fileset.unions [
              ./go.mod ./main.go ./service.go ./registry.go ./handoff.go
              ./workflow_test.go ./web ./assets/brand ./LICENSE
            ];
          };
          vendorHash = null;
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
          default = { type = "app"; program = "${package}/bin/subsoil"; meta.description = "Subsoil file browsing and application handoffs"; };
        });
      devShells = eachSystem (system:
        let pkgs = import nixpkgs { inherit system; };
        in { default = pkgs.mkShell {
          packages = with pkgs; [ go oras jq nodejs python3Packages.ansible-core ];
        }; });
      checks = eachSystem (system: { build = packageFor system; });
    };
}
