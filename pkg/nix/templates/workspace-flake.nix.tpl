{{- define "workspace-flake" }}

{{- $nixpkgsList := .NixPkgsCommits }}
{{- $packagesMap := .PackagesMap }}
{{- $librariesMap := .LibrariesMap }}
{{- $urlPackages := .URLPackages }}
{{- $projectDir := .WorkspaceDir }}
{{- $nixpkgsDefaultCommit := .NixPkgsDefaultCommit -}}
{{- $builds := .Builds -}}

{{- $useProfileFlake := .UseProfileFlake }}
{{- $profileFlakeDir := .ProfileFlakeDir }}

{
  description = "nixy project development workspace";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/{{$nixpkgsDefaultCommit}}";
    flake-utils.url = "github:numtide/flake-utils/11707dc2f618dd54ca8739b309ec4fc024de578b";

    {{- if $useProfileFlake }}
    profile-flake.url = "{{$profileFlakeDir}}";
    {{- end }}
    
    {{- range $_, $v := $nixpkgsList }}
    nixpkgs_{{slice $v 0 7}}.url = "github:nixos/nixpkgs/{{$v}}";
    {{- end }}
  };

  outputs = {
      self, nixpkgs, flake-utils,

      {{- if $useProfileFlake }}
      profile-flake,
      {{- end }}

      {{- range $_, $v := $nixpkgsList -}}
      nixpkgs_{{slice $v 0 7}},
      {{- end }}
    }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system; 
          config.allowUnfree = true;
        };

        {{- range $k := $nixpkgsList -}}
        pkgs_{{slice $k 0 7}} = import nixpkgs_{{slice $k 0 7}} {
          inherit system;
          config.allowUnfree = true;
        };
        {{- end }}

        archMap = {
          "x86_64" = "amd64";
          "aarch64" = "arm64";
        };

        arch = builtins.getAttr (builtins.elemAt (builtins.split "-" system) 0) archMap;
        os = builtins.elemAt (builtins.split "-" system) 2;

        packages = [
          {{- range $k, $v := $packagesMap }}
          {{- range $item := $v }}
          pkgs_{{slice $k 0 7}}.{{$item}}
          {{- end }}
          {{- end }}
        ];

        libraries = pkgs.lib.makeLibraryPath [
          {{- range $k, $v := $librariesMap }}
          {{- range $item := $v }}
          pkgs_{{slice $k 0 7}}.{{$item}}
          {{- end }}
          {{- end }}
        ];
        
        # Custom URL packages
        urlPackages = [
          {{- range $pkg := $urlPackages }}
          (pkgs.stdenv.mkDerivation rec {
            name = "{{$pkg.Name}}";
            pname = "{{$pkg.Name}}";
            src = pkgs.fetchurl {
              url = "{{$pkg.URL}}";
              {{- if $pkg.Sha256 }}
              sha256 = "{{$pkg.Sha256}}";
              {{- else }}
              sha256 = pkgs.lib.fakeSha256;  # Will error and show correct hash
              {{- end }}
            };
            nativeBuildInputs = with pkgs; [
              unzip p7zip unrar xz gzip bzip2 zstd lzip
            ];
            unpackPhase = ''
              echo ">> Detecting archive type for $src"
              mime=$(file -b --mime-type "$src")
              echo ">> Got: $mime"

              try_tar() {
                if tar tf "$src" >/dev/null 2>&1; then
                  echo ">> Extracting tar archive"
                  tar xf "$src"
                  return 0
                fi
                return 1
              }

              case "$mime" in
                application/gzip|application/x-gzip|application/x-xz|application/x-bzip2|application/x-zstd)
                  if ! try_tar; then
                    echo ">> Not a tarball, using decompressor directly"
                    case "$mime" in
                      application/gzip|application/x-gzip) gunzip -k "$src" ;;
                      application/x-bzip2) bunzip2 -k "$src" ;;
                      application/x-xz) xz -d -k "$src" ;;
                      application/x-zstd) unzstd -k "$src" ;;
                    esac
                  fi
                  ;;
                application/x-tar|application/x-gtar)
                  tar xf "$src"
                  ;;
                application/zip)
                  unzip "$src"
                  ;;
                application/x-7z-compressed)
                  7z x "$src"
                  ;;
                application/x-rar)
                  unrar x "$src"
                  ;;
                application/x-executable)
                  # INFO: renaming the script as per the tool name, as it is a one off binary
                  cp "$src" ./$name
                  chmod +x $name
                  ;;
                *)
                  echo "!! Unknown archive type: $mime"
                  echo "Falling back to copying..."
                  cp -r "$src" .
                  ;;
              esac
            '';
            installPhase = ''
              mkdir -p $out/bin
              find . -type f -executable ! -name "*.so*" -exec cp {} $out/bin/ \;
            '';
          })

          {{- end }}
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          buildInputs = {{- if $useProfileFlake }} profile-flake.devShells.${system}.default.buildInputs ++ {{- end }} packages ++ urlPackages;

          shellHook = ''
            if [ -n "${libraries}" ]; then
              export LD_LIBRARY_PATH="${libraries}:$LD_LIBRARY_PATH"
            fi

            if [ -e shell-hook.sh ]; then
              source "shell-hook.sh"
            fi

            if [ "$NIXY_BUILD_HOOK" = "true" ] && [ -e build-hook.sh ]; then
              source "build-hook.sh"
            fi

            cd {{$projectDir}}
          '';
        };

        {{- range $name, $build := $builds }}
        packages.{{$name}} = let
            closure = pkgs.buildEnv {
              name = "build-env";
              paths = [
                {{- range $k, $v := $build.PackagesMap }}
                {{- range $item := $v }}
                pkgs_{{slice $k 0 7}}.{{$item}}
                {{- end }}
                {{- end }}
              ];
            };
          in pkgs.stdenv.mkDerivation {
            name = "{{$name}}";
            nativeBuildInputs = with pkgs; [
              {{- /* INFO: with uutils, date lib is missing nanosecond support */}}
              {{- /* uutils-coreutils-noprefix */}}
              coreutils-full
            ];
            SOURCE_DATE_EPOCH = "0";
            src = [
              closure

              {{- range $v := $build.Paths }}
              {{$v}}
              {{- end }}
            ];

            unpackPhase = ":";

            installPhase = ''
              mkdir -p $out

              shopt -s extglob
              for item in $src; do
                if [ -d "$item" ]; then
                  result=$(stripHash "$item" )
                  echo "item: $item, result: $result"
                  cp -r "$item"/!(share) $out
                else
                  result=$(stripHash "$item" )
                  echo "item: $item, result: $result"
                  cp "$item" $out/$(stripHash "$item")
                fi
              done
            '';
          };

        {{- end }}
      }
    );
}

{{- end }}
