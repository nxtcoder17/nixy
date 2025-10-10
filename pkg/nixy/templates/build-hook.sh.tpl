{{- define "build-hook" }}

{{- $projectDir := .ProjectDir }}
{{- $buildTarget := .BuildTarget }}

set -e

{{- range $p := .CopyPaths }}
mkdir -p $(dirname {{$p}})
cp -r {{$projectDir}}/{{$p}} ./$(dirname {{$p}})
{{- end }}

dir="{{$projectDir}}/.builds/{{$buildTarget}}"
mkdir -p $dir

nix build .#{{$buildTarget}} --no-link -o $dir/app

{{- range $p := .CopyPaths }}
rm -rf {{$p}}
{{- end }}

pushd $dir
rm -rf nix
mkdir -p ./nix/store
cp -r $(nix path-info --recursive ./app) ./nix/store

chown $EUID -R ./nix
chmod 700 -R ./nix
popd

{{- end }}
