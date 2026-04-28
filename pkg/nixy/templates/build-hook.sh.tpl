{{- define "build-hook" }}

{{- $projectDir := .ProjectDir }}
{{- $buildTarget := .BuildTarget }}

set -e

{{- if .Command }}
workspace_dir="$PWD"
cd {{$projectDir}}

{{$command := .Command}}
{{$command}}

cd "$workspace_dir"
{{- end }}

{{- range $p := .CopyPaths }}
mkdir -p $(dirname {{$p}})
cp -r {{$projectDir}}/{{$p}} ./$(dirname {{$p}})
{{- end }}

dir="{{$projectDir}}/.dist/{{$buildTarget}}"
mkdir -p $dir

nix build .#{{$buildTarget}} --no-link -o $dir/app
readlink -f $dir/app > $dir/app-store-path

{{- range $p := .CopyPaths }}
rm -rf {{$p}}
{{- end }}

pushd $dir > /dev/null
rm -rf nix
mkdir -p ./nix/store
cp -r $(nix path-info --recursive ./app) ./nix/store

chown $EUID -R ./nix
chmod 700 -R ./nix
popd > /dev/null

{{- end }}
