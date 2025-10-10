{{- define "profile-nixy.yml" }}
{{- $nixpkgsCommit := .NixPkgsCommit }}
nixpkgs: {{$nixpkgsCommit}}

packages:
  - coreutils-full
  - unixtools.whereis
  - which
  - ncurses

  # your other packages
{{- end }}
