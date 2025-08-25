{{- $nixPkgs := .nixPkgs }}
{{- $nightlyPkgs := .nightlyPkgs }}
{{- $projectDir := .projectDir }}
{{- $profileDir := .profileDir }}
{
  description = "nixy project development workspace";

  inputs = {
    flake-utils.url = "github:numtide/flake-utils";
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";

    profile-flake = {
      url = "path:{{$profileDir}}";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };

    {{- range $k, $_ := $nixPkgs }}
    nixpkgs_{{slice $k 0 7}}.url = "github:nixos/nixpkgs/{{$k}}";
    {{- end }}
  };

  outputs = {
      self, nixpkgs, flake-utils,
      {{- range $k, $_ := $nixPkgs -}}
      nixpkgs_{{slice $k 0 7}},
      {{- end }}
      profile-flake
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

          buildInputs = with pkgs; [
              {{- range $_, $v := $nightlyPkgs -}}
              {{$v}}
              {{- end }}
            ]
            ++ profile-flake.devShells.${system}.default.buildInputs
            ++ [
              {{- range $k, $v := $nixPkgs -}}
              {{- range $pkg := $v }}
              pkgs_{{slice $k 0 7}}.{{$pkg}}
              {{- end }}
              {{- end }}
            ];

          shellHook = ''
            pushd {{$projectDir}}
          '';
        };
      }
    );
}

