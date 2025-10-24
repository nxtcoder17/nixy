{{- define "workspace-flake" }}

{{- $nixpkgsList := .NixPkgsCommitsList }}
{{- $nixpkgsMap := .NixPkgsCommitsMap }}
{{- $packagesMap := .PackagesMap }}
{{- $librariesMap := .LibrariesMap }}
{{- $urlPackages := .URLPackages }}
{{- $projectDir := .WorkspaceDir }}
{{- $builds := .Builds -}}
{{- $osArch := .OSArch }}

{{- $nixpkgsDefaultCommit := index $nixpkgsList 0 -}}
{
  description = "nixy project development workspace";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils/11707dc2f618dd54ca8739b309ec4fc024de578b";

    {{- range $k := $nixpkgsList }}
    nixpkgs_{{$k}}.url = "github:nixos/nixpkgs/{{index $nixpkgsMap $k}}";
    {{- end }}
  };

  outputs = {
      self, flake-utils,
      {{- range $v := $nixpkgsList -}}
      nixpkgs_{{$v}},
      {{- end }}
    }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs_{{$nixpkgsDefaultCommit}} {
          inherit system;
          config.allowUnfree = true;
        };

        {{ range $k := $nixpkgsList -}}
        pkgs_{{$k}} = import nixpkgs_{{$k}} {
          inherit system;
          config.allowUnfree = true;
        };
        {{- end }}

        packages = [
          {{- range $k := $nixpkgsList -}}
          {{- range $item := index $packagesMap $k }}
          pkgs_{{$k}}.{{$item}}
          {{- end }}
          {{- end }}
        ] ++ (pkgs.lib.optionals pkgs.stdenv.isLinux [ pkgs.glibcLocales ]);

        libraries = pkgs.lib.makeLibraryPath [
          {{- range $k := $nixpkgsList -}}
          {{- range $item := index $librariesMap $k }}
          pkgs_{{$k}}.{{$item}}
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
              sha256 = let
                  shaMap = builtins.fromJSON ''{{$pkg.Sha256 | toJson }}'';
                in
                  if builtins.hasAttr "{{$osArch}}" shaMap
                  then builtins.getAttr "{{$osArch}}" shaMap
                  else throw "No sha256 found for OS/Arch '{{$osArch}}'. Please add the correct hash to the Sha256 map.";
            };
            nativeBuildInputs = with pkgs; [
              unzip p7zip unrar xz gzip bzip2 zstd lzip
              patchelf autoPatchelfHook
            ];
            unpackPhase = ''
              echo ">> Detecting archive type for $src"
              mime=$(file -b --mime-type "$src")
              echo ">> Got: $mime"

              try_tar() {
                if tar tf "$src" >/dev/null 2>&1; then
                  echo ">> Extracting tar archive"
                  tar xf "$src"
                  ls -al .
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
              shopt -s extglob
              tmp_file=$(mktemp --tmpdir=/tmp)
              echo "tmp file: $tmp_file"
              ls | grep -v 'env-vars' > $tmp_file

              total_count=$(cat $tmp_file | wc -l)
              echo "total count: $total_count"

              if [ $total_count -eq 1 ] ; then
                filepath=$(sed "1q;d" $tmp_file)
                if [ -d "$filepath" ]; then
                  cp -r $filepath/* $out
                else
                  cp -r $filepath $out/bin
                fi
              else
                cp -r . $out
              fi

              echo "[##] printing contents of $out now"
              ls -al $out
            '';
          })

          {{- end }}
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          buildInputs = packages ++ urlPackages;

          shellHook = ''
            {{- /* INFO: because glibcLocales is a linux only package, and causes nixy shell to break on macos */}}
            ${
              if pkgs.stdenv.isLinux
              then ''export LOCALE_ARCHIVE=${pkgs.glibcLocales}/lib/locale/locale-archive''
              else ''''
            }

            if [ -z "$LANG" ]; then
              # INFO: if LANG env var unset, set it to en_US.UTF-8
              export LANG="en_US.UTF-8"
            fi

            if [ ! -e "/usr/bin" ]; then
              # INFO: this ensures, we always have /usr/bin/env
              ln -sf ${pkgs.coreutils}/* /usr
            fi

            # INFO: it seems like many tools have hardcoded value for /bin/sh, so we need to make sure that /bin/sh exists
            if [ ! -e "/bin/sh" ]; then
              mkdir -p /bin
              ln -sf $(which bash) /bin/sh
            fi

            if [ -n "${libraries}" ]; then
              export LD_LIBRARY_PATH="${libraries}:$LD_LIBRARY_PATH"
            fi

            {{- range $k, $v := .EnvVars }}
            export {{$k}}="{{$v}}"
            {{- end }}

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
                pkgs_{{$k}}.{{$item}}
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
            {{- /* SOURCE_DATE_EPOCH = "0"; */}}
            src = [
              closure

              {{- range $v := $build.Paths }}
              {{$v}}
              {{- end }}
            ];

            unpackPhase = ":";

            installPhase = ''
              mkdir -p "$out"

              shopt -s extglob
              for item in $src; do
                if [ -d "$item" ]; then
                  # INFO: stripHash just removes nix hash part from the given name
                  echo "[#] copying dir: $item"
                  result=$(stripHash "$item" )
                  cp -r "$item"/!(share) $out
                else
                  echo "[#] copying file: $item"
                  result=$(stripHash "$item" )
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
