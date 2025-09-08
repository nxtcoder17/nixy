{{- range $k, $v := $envVars }}
export {{$k}}='{{$v}}'
{{- end }}

