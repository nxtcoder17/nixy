{{- $nixPkgs := .nixPkgs }}
{{- $projectDir := .projectDir }}
{{- $profileDir := .profileDir }}
{{- $nixpkgsCommit := .nixpkgsCommit }}
{
  description = "nixy project development workspace";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/{{$nixpkgsCommit}}";
    flake-utils.url = "github:numtide/flake-utils/11707dc2f618dd54ca8739b309ec4fc024de578b";

    {{- if $profileDir }}
    profile-flake = {
      url = "path:{{$profileDir}}";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
    {{- end }}

    {{- range $k, $_ := $nixPkgs }}
    nixpkgs_{{slice $k 0 7}}.url = "github:nixos/nixpkgs/{{$k}}";
    {{- end }}
  };

  outputs = {
      self, nixpkgs, flake-utils,
      {{- range $k, $_ := $nixPkgs -}}
      nixpkgs_{{slice $k 0 7}},
      {{- end }}
      {{- if $profileDir }} profile-flake {{- end }}
    }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs {
          inherit system; 
          config.allowUnfree = true;
        };

        {{ range $k, $_ := $nixPkgs -}}
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
      in
      {
        devShells.default = pkgs.mkShell {
          # hardeningDisable = [ "all" ];

          buildInputs = {{- if $profileDir}}profile-flake.devShells.${system}.default.buildInputs
            ++ {{- end }} [
              {{- range $k, $v := $nixPkgs -}}
              {{- range $pkg := $v }}
              pkgs_{{slice $k 0 7}}.{{$pkg}}
              {{- end }}
              {{- end }}
            ];

          shellHook = ''
            cd {{$projectDir}}
          '';
        };
      }
    );
}

