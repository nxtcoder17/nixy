package nix

import (
	_ "embed"
)

type WorkspaceFlakeParams struct {
	NixPkgsDefaultCommit string
	NixPkgsCommits       []string

	PackagesMap  map[string][]string
	LibrariesMap map[string][]string
	URLPackages  []URLPackage

	ProfileFlakeDir string
	WorkspaceDir    string

	Builds map[string]WorkspaceFlakePackgeBuild
}

type WorkspaceFlakePackgeBuild struct {
	PackagesMap map[string][]string
	Paths       []string
}

type ProfileFlakeParams struct {
	NixPkgsCommit string
}
