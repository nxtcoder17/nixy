package nixy

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"gopkg.in/yaml.v3"
)

type Mode string

const (
	LocalMode          Mode = "local"
	LocalIgnoreEnvMode Mode = "local-ignore-env"
	DockerMode         Mode = "docker"
	BubbleWrapMode     Mode = "bubblewrap"
)

func (m Mode) String() string {
	return string(m)
}

type NixyMount struct {
	Source      string `yaml:"source"`
	Destination string `yaml:"dest"`
	ReadOnly    bool   `yaml:"readonly,omitempty"`
}

type NixPkgsMap map[string]string

func (m NixPkgsMap) List() []string {
	keys := make([]string, 0, len(m))
	keys = append(keys, "default")
	for k := range m {
		if k != "default" {
			keys = append(keys, k)
		}
	}

	slices.Sort(keys[1:])
	return keys
}

func (m NixPkgsMap) DefaultCommit() string {
	return "default"
}

type Nixy struct {
	NixPkgs   NixPkgsMap           `yaml:"nixpkgs"`
	Packages  []*NormalizedPackage `yaml:"packages"`
	Libraries []string             `yaml:"libraries,omitempty"`

	Env map[string]string `yaml:"env,omitempty"`

	OnShellEnter string `yaml:"onShellEnter,omitempty"`

	// OnShellExit is not used as of now, will try to use it in future
	OnShellExit string `yaml:"onShellExit,omitempty"`

	Builds map[string]Build `yaml:"builds,omitempty"`

	// Mount is applicable only on bubblewrap and docker modes
	Mounts []NixyMount `yaml:"mounts,omitempty"`

	// AUTO FILLED
	sha256Sum string `yaml:"-"`

	// rawNode holds the original yaml.Node tree for comment preservation
	rawNode *yaml.Node `yaml:"-"`
}

func (n *Nixy) debug() {
	b, err := yaml.Marshal(n)
	if err != nil {
		panic(err)
	}

	fmt.Printf("\n%s\n", b)
}

type NixyWrapper struct {
	Context *Context

	hasHashChanged bool
	executorArgs   *ExecutorArgs `yaml:"-"`
	sync.Mutex     `yaml:"-"`
	Logger         *slog.Logger  `yaml:"-"`
	runtimePaths   *RuntimePaths `yaml:"-"` // Always set (workspace infrastructure)
	profile        *Profile      `yaml:"-"` // Only set when NIXY_USE_PROFILE=true
	profileNixy    *Nixy         `yaml:"-"` // Only set when NIXY_USE_PROFILE=true

	PWD string

	*Nixy
}

type ExecutorArgs struct {
	NixBinaryMountedPath  string
	ProfileDirMountedPath string
	FakeHomeMountedPath   string
	NixDirMountedPath     string

	WorkspaceFlakeDirHostPath    string
	WorkspaceFlakeDirMountedPath string

	EnvVars executorEnvVars
}

type Build struct {
	Packages []*NormalizedPackage `yaml:"packages"`
	Paths    []string             `yaml:"paths"`
}

type InShellNixy struct {
	PWD    string `yaml:"-"`
	Logger *slog.Logger
	Nixy
}

func LoadInNixyShell(parent context.Context) (*InShellNixy, error) {
	workspaceDir, ok := os.LookupEnv("NIXY_WORKSPACE_DIR")
	if !ok {
		return nil, fmt.Errorf("in nixy shell, NIXY_WORKSPACE_DIR must be defined")
	}

	nixyFile := filepath.Join(workspaceDir, "nixy.yml")

	b, err := os.ReadFile(nixyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read nixy file (%s): %w", nixyFile, err)
	}

	nixy := InShellNixy{
		Logger: slog.Default(),
		PWD:    workspaceDir,
		Nixy:   Nixy{},
	}

	if err := yaml.Unmarshal(b, &nixy.Nixy); err != nil {
		return nil, err
	}

	return &nixy, nil
}

func parseAndSyncNixyFile(_ context.Context, file string) (*Nixy, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read nixy file (%s): %w", file, err)
	}

	// Parse as yaml.Node to preserve comments and structure
	var rootNode yaml.Node
	if err := yaml.Unmarshal(b, &rootNode); err != nil {
		return nil, err
	}

	// Also decode into struct for processing
	var nixyCfg Nixy
	if err := rootNode.Decode(&nixyCfg); err != nil {
		return nil, err
	}

	// Store the raw node for later sync
	nixyCfg.rawNode = &rootNode

	if _, ok := nixyCfg.NixPkgs["default"]; !ok {
		return nil, fmt.Errorf("nixy.yml must have a nixpkgs.default key, containing a nixpkgs hash")
	}

	hasher := sha256.New()
	hasher.Write([]byte(os.Getenv("NIXY_VERSION")))
	// Use NIXY_EXECUTOR to match Context.NixyMode, ensuring different executor modes
	// always result in distinct workspace hashes
	hasher.Write([]byte(os.Getenv("NIXY_EXECUTOR")))
	hasher.Write(b)
	nixyCfg.sha256Sum = fmt.Sprintf("%x", hasher.Sum(nil))[:7]

	hasPkgUpdates := false

	for i, pkg := range nixyCfg.Packages {
		if pkg == nil {
			continue
		}

		// Fetch SHA256 if not provided
		if pkg.URLPackage != nil {
			osArch := getOSArch()
			v, hasSource := pkg.URLPackage.Sources[osArch]
			if !hasSource || v.URL == "" {
				return nil, fmt.Errorf("URL package %q has no source defined for %s", pkg.URLPackage.Name, osArch)
			}

			if v.SHA256 != "" {
				continue
			}

			hash, err := fetchURLPackageHash(v.URL)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch SHA256 hash for (name: %s, url: %s): %w", pkg.URLPackage.Name, v.URL, err)
			}

			hasPkgUpdates = true

			pkg.URLPackage.Sources[osArch] = URLAndSHA{
				URL:    v.URL,
				SHA256: hash,
			}

			// Update the SHA256 in the raw node tree
			if err := updateSHA256InNode(&rootNode, i, osArch, hash); err != nil {
				slog.Warn("failed to update SHA256 in node tree, will regenerate", "error", err)
				nixyCfg.rawNode = nil
			}
		}
	}

	if hasPkgUpdates {
		if err := nixyCfg.SyncToDisk(file); err != nil {
			return nil, err
		}
	}

	return &nixyCfg, nil
}

func LoadFromFile(parent context.Context, f string) (*NixyWrapper, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to read current working directory: %w", err)
	}

	nc, err := parseAndSyncNixyFile(parent, f)
	if err != nil {
		return nil, err
	}

	ctx, err := NewContext(parent, filepath.Dir(f))
	if err != nil {
		return nil, err
	}

	// Always create runtime paths (needed for workspace flake storage)
	runtimePaths, err := NewRuntimePaths(ctx.NixyProfile)
	if err != nil {
		return nil, err
	}

	hasHashChanged, err := compareAndSaveHash(filepath.Join(flakeDirPath(ctx.NixyProfile), "nixy.yml.sha256"), nc.sha256Sum)
	if err != nil {
		return nil, err
	}

	nixy := NixyWrapper{
		Context:        ctx,
		hasHashChanged: hasHashChanged,
		runtimePaths:   runtimePaths,
		Logger:         slog.Default(),
		PWD:            dir,
		Nixy:           nc,
	}

	// Only load profile configuration when NIXY_USE_PROFILE is enabled
	if ctx.NixyUseProfile {
		profile, err := NewProfile(ctx, ctx.NixyProfile, runtimePaths)
		if err != nil {
			return nil, err
		}
		nixy.profile = profile

		nc, err := parseAndSyncNixyFile(ctx, profile.ProfileNixyYAMLPath)
		if err != nil {
			return nil, err
		}
		hasChanged, err := compareAndSaveHash(filepath.Join(profilePath(ctx.NixyProfile), "nixy.yml.sha256"), nc.sha256Sum)
		if err != nil {
			return nil, err
		}

		nixy.profileNixy = nc
		// If either the workspace or profile nixy.yml has changed, we need to regenerate
		nixy.hasHashChanged = nixy.hasHashChanged || hasChanged
	}

	switch ctx.NixyMode {
	case BubbleWrapMode:
		nixy.executorArgs, err = UseBubbleWrap(ctx, runtimePaths)
		if err != nil {
			return nil, err
		}
	case DockerMode:
		nixy.executorArgs, err = UseDocker(ctx, runtimePaths)
		if err != nil {
			return nil, err
		}
	case LocalMode:
		nixy.executorArgs, err = UseLocal(ctx, runtimePaths)
		if err != nil {
			return nil, err
		}
	}

	if _, ok := nixy.NixPkgs["default"]; !ok {
		return nil, fmt.Errorf("nixy.yml must have a nixpkgs.default key, containing a nixpkgs hash")
	}

	return &nixy, nil
}

func compareAndSaveHash(saveToFile string, sha256Sum string) (bool, error) {
	if err := os.MkdirAll(filepath.Dir(saveToFile), 0o755); err != nil {
		return false, fmt.Errorf("failed to create dir %s: %s", filepath.Dir(saveToFile), err)
	}

	hasHashChanged := true
	if exists(saveToFile) {
		hash, err := os.ReadFile(saveToFile)
		if err != nil {
			return false, fmt.Errorf("failed to read hash file (%s): %w", saveToFile, err)
		}

		slog.Debug("comparing nixy.yml hash", "nixy-file", saveToFile, "file.hash", string(hash), "nixy.hash", sha256Sum)
		hasHashChanged = string(hash) != sha256Sum
	}

	if hasHashChanged {
		slog.Debug("nixy.yml hash changed", "to", sha256Sum)
		if err := os.WriteFile(saveToFile, []byte(sha256Sum), 0o644); err != nil {
			return false, fmt.Errorf("failed to write sha256 hash (path: %s): %w", saveToFile, err)
		}
	}

	return hasHashChanged, nil
}

// SyncToDisk writes the nixy config to disk.
// When rawNode is set, it preserves the original YAML structure (comments, ordering).
// When rawNode is nil, it encodes the struct with deduplication (for new files).
func (nixy *Nixy) SyncToDisk(file string) error {
	if file == "" {
		return fmt.Errorf("required param `file` not provided")
	}

	output, err := os.OpenFile(file,
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0o644,
	)
	if err != nil {
		return err
	}
	defer output.Close()

	encoder := yaml.NewEncoder(output)
	encoder.SetIndent(2)
	defer encoder.Close()

	// rawNode path: preserves comments and user's structure, skips deduplication
	if nixy.rawNode != nil {
		return encoder.Encode(nixy.rawNode)
	}

	// Struct path (new files): encode with deduplication
	upkg := make([]*NormalizedPackage, 0, len(nixy.Packages))
	set := make(map[string]struct{}, len(nixy.Packages))

	for _, pkg := range nixy.Packages {
		if pkg == nil {
			continue
		}

		var key string

		if pkg.NixPackage != nil {
			key = pkg.NixPackage.Name
		}

		if pkg.URLPackage != nil {
			key = pkg.URLPackage.Name
		}

		if _, ok := set[key]; ok {
			continue
		}
		set[key] = struct{}{}
		upkg = append(upkg, pkg)
	}

	nixy.Packages = upkg

	return encoder.Encode(nixy)
}

// YAML Node helper functions for traversing and modifying yaml.Node trees

// findMappingValue finds the value node for a given key in a mapping node.
// Returns nil if the key is not found or the node is not a mapping.
func findMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1]
		}
	}
	return nil
}

// getSequenceItem returns the item at the given index in a sequence node.
// Returns nil if the index is out of range or the node is not a sequence.
func getSequenceItem(node *yaml.Node, index int) *yaml.Node {
	if node == nil || node.Kind != yaml.SequenceNode {
		return nil
	}
	if index < 0 || index >= len(node.Content) {
		return nil
	}
	return node.Content[index]
}

// setOrInsertScalarField sets the value of a scalar field in a mapping node,
// or inserts it after the specified key if it doesn't exist.
// If afterKey is empty or not found, appends to the end.
func setOrInsertScalarField(node *yaml.Node, key, value, afterKey string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}

	// Check if key already exists
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1].Value = value
			return
		}
	}

	// Key doesn't exist, find position to insert after afterKey
	insertIdx := len(node.Content) // default: append to end
	if afterKey != "" {
		for i := 0; i < len(node.Content)-1; i += 2 {
			if node.Content[i].Value == afterKey {
				insertIdx = i + 2 // insert after the value of afterKey
				break
			}
		}
	}

	// Insert at the computed position
	newContent := make([]*yaml.Node, 0, len(node.Content)+2)
	newContent = append(newContent, node.Content[:insertIdx]...)
	newContent = append(newContent,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
	newContent = append(newContent, node.Content[insertIdx:]...)
	node.Content = newContent
}

// updateSHA256InNode updates the sha256 field for a URL package in the yaml.Node tree
func updateSHA256InNode(root *yaml.Node, pkgIndex int, osArch, hash string) error {
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return fmt.Errorf("invalid root node")
	}

	docNode := root.Content[0]
	if docNode.Kind != yaml.MappingNode {
		return fmt.Errorf("expected mapping node")
	}

	packagesNode := findMappingValue(docNode, "packages")
	if packagesNode == nil || packagesNode.Kind != yaml.SequenceNode {
		return fmt.Errorf("packages not found or not a sequence")
	}

	pkgNode := getSequenceItem(packagesNode, pkgIndex)
	if pkgNode == nil {
		return fmt.Errorf("package index %d out of range", pkgIndex)
	}
	if pkgNode.Kind != yaml.MappingNode {
		return fmt.Errorf("package is not a mapping (might be a string package)")
	}

	sourcesNode := findMappingValue(pkgNode, "sources")
	if sourcesNode == nil || sourcesNode.Kind != yaml.MappingNode {
		return fmt.Errorf("sources not found or not a mapping")
	}

	archNode := findMappingValue(sourcesNode, osArch)
	if archNode == nil || archNode.Kind != yaml.MappingNode {
		return fmt.Errorf("osArch %s not found or not a mapping", osArch)
	}

	// Set or insert sha256 after url
	setOrInsertScalarField(archNode, "sha256", hash, "url")
	return nil
}

func InitNixyFile(parent context.Context, dest string) error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx, err := NewContext(parent, dir)
	if err != nil {
		return err
	}

	runtimePaths, err := NewRuntimePaths(ctx.NixyProfile)
	if err != nil {
		return err
	}

	profile, err := NewProfile(ctx, ctx.NixyProfile, runtimePaths)
	if err != nil {
		return err
	}

	n := &Nixy{
		NixPkgs: map[string]string{
			"default": profile.NixPkgsCommitHash,
		},
	}
	return n.SyncToDisk(dest)
}
