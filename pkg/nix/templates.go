package nix

import (
	_ "embed"
)

//go:embed templates/profile-flake.nix.tpl
var templateProfileFlake string

//go:embed templates/project-flake.nix.tpl
var templateProjectFlake string
