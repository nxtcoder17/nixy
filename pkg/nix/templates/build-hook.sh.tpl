{{- define "build-hook" }}

{{- $projectDir := .ProjectDir }}
{{- $buildTarget := .BuildTarget }}

{{- range $p := .CopyPaths }}
mkdir -p $(dirname {{$p}})
cp -r {{$projectDir}}/{{$p}} ./$(dirname {{$p}})
{{- end }}

set -e
mkdir -p {{$projectDir}}/.builds/
{{- /* nix build --quiet --quiet .#{{$buildTarget}} -o {{$projectDir}}/.builds/{{$buildTarget}} */}}
nix build .#{{$buildTarget}} -o {{$projectDir}}/.builds/{{$buildTarget}}

{{- range $p := .CopyPaths }}
rm -rf {{$p}}
{{- end }}

pushd {{$projectDir}}/.builds
rm -rf nix-store
mkdir -p nix-store
cp -r $(nix path-info --recursive ./{{$buildTarget}}) ./nix-store

chown $EUID -R ./nix-store
chmod 700 -R ./nix-store
popd

{{- end }}
