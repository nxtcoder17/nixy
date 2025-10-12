{
  description = "development workspace";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (
      system: let
        pkgs = import nixpkgs {
          inherit system;
          # config.allowUnfree = true;
        };

        archMap = {
          "x86_64" = "amd64";
          "aarch64" = "arm64";
        };

        arch = builtins.getAttr (builtins.elemAt (builtins.split "-" system) 0) archMap;
        os = builtins.elemAt (builtins.split "-" system) 2;
      in {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          nativeBuildInputs = with pkgs; [
            (stdenv.mkDerivation rec {
              name = "run";
              pname = "run";
              src = fetchurl {
                url = "https://github.com/nxtcoder17/Runfile/releases/download/v1.5.4/run-${os}-${arch}";
                sha256 = builtins.getAttr "${os}/${arch}" {
                  "linux/amd64" = "j/0q+cNdt2ltFIpCgnenvZGX1GEJ5ZKBrRfskalhO5c=";
                  "linux/arm64" = "BsI1cFNG/wEGa33HZiG+Mt/iSaA8kkPyrQX+lbGrMaM=";
                  "darwin/amd64" = "VDroUq7dOvHa5rWK9N01Mv6aqUfXcVrk/NRXvGiYzAk=";
                  "darwin/arm64" = "iltkmz3G2zeSs04La1xB1IcvfzG2g6ssisET5skhs2U=";
                };
              };
              unpackPhase = ":";
              installPhase = ''
                mkdir -p $out/bin
                cp $src $out/bin/$name
                chmod +x $out/bin/$name
              '';
            })

            # your packages here
            go
            nginx
            coreutils
          ];

          shellHook = ''
            export PATH="$PWD/bin:$PATH"
          '';
        };

        packages.default = pkgs.buildGoModule {
          pname = "nixy";
          version = "dev";
          src = ./.;

          vendorHash = "sha256-jvWfx7cvEsmofeV7pDtNkzKM50QVHa9pb0rtzeyu7ro=";

          subPackages = ["cmd"];

          ldflags = [
            "-s"
            "-w"
            "-X main.Version=dev"
          ];

          postInstall = ''
            mv $out/bin/cmd $out/bin/nixy
          '';

          meta = with pkgs.lib; {
            description = "An approachable nix based development workspace setup tool";
            homepage = "https://github.com/nxtcoder17/nixy";
            license = licenses.mit;
            maintainers = [];
          };
        };
      }
    );
}
