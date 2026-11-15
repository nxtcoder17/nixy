{
  description = "development workspace";

  inputs = {
    # pinned to nixy.yml's nixpkgs.default — the same snapshot the product
    # shells resolve against, so image and user builds see the same nixpkgs
    nixpkgs.url = "github:nixos/nixpkgs/050e09e091117c3d7328c7b2b7b577492c43c134";
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

        # ghcr.io/nxtcoder17/nix — the nix runtime image. Published by
        # .github/workflows/docker.yml, which only runs when flake.nix/flake.lock
        # change on the default branch, so the nix version is whatever nixpkgs
        # snapshot flake.lock pins.
        #
        # The image is exactly the closure of the packages listed below; there is
        # no post-hoc cleanup step, so nothing can be in the image that isn't
        # declared here.
        packages.nix-image =
          let
            nixConf = pkgs.writeTextDir "etc/nix/nix.conf" ''
              experimental-features = nix-command flakes
              # no nixbld users are shipped; builds run as the calling user
              # (root in Dockerfile.build, the --user uid in nixy docker mode)
              build-users-group =
              # docker containers may not have user namespaces available
              # (the official nixos/nix image does the same)
              sandbox = false
            '';
          in
          pkgs.dockerTools.buildLayeredImage {
            name = "nix";
            tag = "latest";

            contents = with pkgs; [
              dockerTools.usrBinEnv # /usr/bin/env
              dockerTools.binSh # /bin/sh -> bash
              dockerTools.caCertificates # ca bundle at the usual /etc/ssl paths
              dockerTools.fakeNss # minimal /etc/passwd + /etc/group (root, nobody)
              nix
              nixConf # /etc/nix/nix.conf
              bashInteractive
              coreutils
              curl
            ];

            config = {
              Env = [
                "PATH=/bin:/usr/bin"
                "NIX_SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
                "SSL_CERT_FILE=${pkgs.cacert}/etc/ssl/certs/ca-bundle.crt"
                "HOME=/root"
              ];
              Cmd = ["/bin/sh"];
            };

            fakeRootCommands = ''
              # world-writable tmp, a home for root, and a writable store:
              # nixy's docker mode seeds /nix into a named volume and chowns it to
              # the invoking user, which only works if the dirs allow writing
              mkdir -p tmp root home nix/store nix/var/nix
              chmod 1777 tmp
              chmod 0755 nix nix/store nix/var/nix
            '';
          };
      }
    );
}
