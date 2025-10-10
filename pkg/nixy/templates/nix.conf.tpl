{{- define "nix.conf" }}
experimental-features = nix-command flakes
http-connections = 40
connect-timeout = 5
{{- /* filter-syscalls = false */}}
{{- /* sandbox = false */}}
{{- end }}
