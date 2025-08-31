{{- $nixpkgsCommit := .NixPkgsCommit }}
{
  description = "nixy profile flake";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/{{$nixpkgsCommit}}";
    flake-utils.url = "github:numtide/flake-utils/11707dc2f618dd54ca8739b309ec4fc024de578b";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system; 
          config.allowUnfree = true;
        };

        archMap = {
          "x86_64" = "amd64";
          "aarch64" = "arm64";
        };

        arch = builtins.getAttr (builtins.elemAt (builtins.split "-" system) 0) archMap;
        os = builtins.elemAt (builtins.split "-" system) 2;
      in
      {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          buildInputs = with pkgs; [
            uutils-coreutils-noprefix
            ncurses
            # your packages here
          ];

          shellHook = ''
          '';
        };
      }
    );
}
