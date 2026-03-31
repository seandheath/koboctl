{
  description = "koboctl — Kobo e-reader provisioning CLI";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" "aarch64-darwin" "x86_64-darwin" ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f system);
    in {
      packages = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system}; in {
          default = pkgs.buildGoModule {
            pname = "koboctl";
            version = "0.1.0";
            src = ./.;
            # Update vendorHash after running: go mod vendor
            vendorHash = "";
            CGO_ENABLED = "0";
          };
        });

      devShells = forAllSystems (system:
        let pkgs = nixpkgs.legacyPackages.${system}; in {
          default = pkgs.mkShell {
            packages = with pkgs; [
              go
              golangci-lint
              gotools      # goimports, etc.
              gnumake
            ];
            shellHook = ''
              export CGO_ENABLED=0
            '';
          };
        });
    };
}
