
{
  description = "nixy project development workspace";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    profile-flake.url = builtins.Getenv "NIXY_PROFILE_DIR";

    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    nixpkgs_b74a30d.url = "github:nixos/nixpkgs/b74a30dbc0a72e20df07d43109339f780b439291"
  };

  outputs = {
      self, nixpkgs, flake-utils,nixpkgs_b74a30d,
      profile-flake
    }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system; 
          config.allowUnfree = true;
        };

        pkgs_b74a30d = import nixpkgs_b74a30d {
          inherit system;
          config.allowUnfree = true;
        };

        profileShell = profile-flake.devShells.${system}.default;

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

          buildInputs = profileShell.buildInputs ++ 
            [
              pkgs_b74a30d.python314
              pkgs_b74a30d.curl
            ] ++
            with pkgs; [
            ];

          shellHook = ''
          '';
        };
      }
    );
}

