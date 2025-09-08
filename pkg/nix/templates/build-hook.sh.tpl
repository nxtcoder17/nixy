{{- define "build-hook" }}

{{- $projectDir := .ProjectDir }}
{{- $buildTarget := .BuildTarget }}

{{- range $p := .CopyPaths }}
mkdir -p $(dirname {{$p}})
cp -r {{$projectDir}}/{{$p}} ./$(dirname {{$p}})
{{- end }}

set -e
mkdir -p {{$projectDir}}/.builds
nix build --quiet --quiet .#{{$buildTarget}} -o {{$projectDir}}/.builds/{{$buildTarget}}

{{- range $p := .CopyPaths }}
rm -rf {{$p}}
{{- end }}

{{- end }}
