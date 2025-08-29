{{- $nixpkgsList := .nixpkgsCommitList }}
{{- $packagesMap := .packagesMap }}
{{- $librariesMap := .librariesMap }}

{{- $projectDir := .projectDir }}
{{- $profileDir := .profileDir }}
{{- $nixpkgsDefaultCommit := .nixpkgsDefaultCommit }}

{
  description = "nixy project development workspace";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/{{$nixpkgsDefaultCommit}}";
    flake-utils.url = "github:numtide/flake-utils/11707dc2f618dd54ca8739b309ec4fc024de578b";

    {{- if $profileDir }}
    profile-flake = {
      url = "path:{{$profileDir}}";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
    {{- end }}

    {{- range $_, $v := $nixpkgsList }}
    nixpkgs_{{slice $v 0 7}}.url = "github:nixos/nixpkgs/{{$v}}";
    {{- end }}
  };

  outputs = {
      self, nixpkgs, flake-utils,
      {{- range $_, $v := $nixpkgsList -}}
      nixpkgs_{{slice $v 0 7}},
      {{- end }}
      {{- if $profileDir }} profile-flake {{- end }}
    }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system; 
          config.allowUnfree = true;
        };

        {{ range $k, $_ := $packagesMap -}}
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

        libraries = pkgs.lib.makeLibraryPath [
          {{- range $k, $v := $librariesMap -}}
          {{- range $pkg := $v }}
          pkgs_{{slice $k 0 7}}.{{$pkg}}
          {{- end }}
          {{- end }}
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          buildInputs = {{- if $profileDir}}profile-flake.devShells.${system}.default.buildInputs
            ++ {{- end }} [
              {{- range $k, $v := $packagesMap -}}
              {{- range $pkg := $v }}
              pkgs_{{slice $k 0 7}}.{{$pkg}}
              {{- end }}
              {{- end }}
            ];

          shellHook = ''
            if [ -n "${libraries}" ]; then
              export LD_LIBRARY_PATH="${libraries}:$LD_LIBRARY_PATH"
            fi
            if [ -e shell-hook.sh ]; then
              source "shell-hook.sh"
            fi
            cd {{$projectDir}}
          '';
        };
      }
    );
}

