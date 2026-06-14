package nixy2

import (
	"github.com/nxtcoder17/go.errors"
	"github.com/nxtcoder17/nixy/internal/app"

	"crypto/sha256"
	"fmt"
	yamlAST "github.com/nxtcoder17/nixy/pkg/yaml-ast"
	"os"
	"path/filepath"
	"strings"
)

type Nixy struct {
	NixPkgs   map[string]string    `yaml:"nixpkgs"`
	Packages  []*NormalizedPackage `yaml:"packages"`
	Libraries []string             `yaml:"libraries,omitempty"`

	Env map[string]string `yaml:"env,omitempty"`

	// OnShellEnter runs at the final step in nixy shell lifecycle
	OnShellEnter string `yaml:"onShellEnter,omitempty"`

	// OnShellExit is not used as of now, will try to use it in future
	OnShellExit string `yaml:"onShellExit,omitempty"`

	Builds map[string]Build `yaml:"builds,omitempty"`

	// Mounts are only for non-local execution modes
	Mounts []NixyMount `yaml:"mounts,omitempty"`
}

type NixyWrapper struct {
	Nixy

	// AUTO FILLED
	sha256Sum string `yaml:"-"`
}

func hashNixyFile(appCtx *app.Context, content []byte) string {
	h := sha256.New()
	h.Write([]byte(appCtx.AppVersion))
	h.Write([]byte(appCtx.NixyMode))
	h.Write(content)

	return fmt.Sprintf("%x", h.Sum(nil))
}

func parseAndSyncNixyFile(appCtx *app.Context, file string) (*NixyWrapper, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return nil, errors.New("failed to read nixy file").Wrap(err).KV("file", file)
	}

	parser, err := yamlAST.NewParser(b)
	if err != nil {
		return nil, err
	}

	var nw NixyWrapper
	if err := parser.Decode(&nw.Nixy); err != nil {
		return nil, err
	}

	if nw.Nixy.NixPkgs["default"] == "" {
		return nil, errors.New("nixy.yml must have a nixpkgs.default key, containing a nixpkgs hash").KV("nixpkgs-map", nw.Nixy.NixPkgs)
	}

	nw.sha256Sum = hashNixyFile(appCtx, b)

	hasPkgUpdates := false

	for i, pkg := range nw.Packages {
		if pkg == nil {
			continue
		}

		// Fetch SHA256 if not provided
		if pkg.URLPackage != nil {
			osArch := getOSArch()

			v, hasSource := pkg.URLPackage.Sources[osArch]
			if !hasSource || v.URL == "" {
				return nil, errors.New("URL package has no source os/arch defined").Wrap(err).KV("name", pkg.URLPackage.Name, "os/arch", osArch)
			}

			if v.SHA256 != "" {
				continue
			}

			hash, err := fetchURLPackageHash(appCtx, v.URL)
			if err != nil {
				return nil, errors.New("failed to fetch SHA256 hash").Wrap(err).KV("name", pkg.URLPackage.Name, "url", v.URL)
			}

			hasPkgUpdates = true

			pkg.URLPackage.Sources[osArch] = URLAndSHA{
				URL:    v.URL,
				SHA256: hash,
			}

			// Update the SHA256 in the raw node tree
			if err := parser.ApplyPatches([]yamlAST.PatchOp{
				{
					Op:    "add",
					Path:  fmt.Sprintf("/packages/%d/sources/%s/sha256", i, strings.ReplaceAll(osArch, "/", "~1")),
					Value: v,
				},
			}); err != nil {
				return nil, errors.New("failed to apply patches").Wrap(err)
			}
		}
	}

	if hasPkgUpdates {
		if err := nw.SyncToDisk(file); err != nil {
			return nil, err
		}
	}

	return &nw, nil
}

// SyncToDisk writes the nixy config to disk.
// When rawNode is set, it preserves the original YAML structure (comments, ordering).
// When rawNode is nil, it encodes the struct with deduplication (for new files).
func (nixy *NixyWrapper) SyncToDisk(file string) error {
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

func LoadFromFile(appCtx *app.Context, f string) (*NixyWrapper, error) {
	nw, err := parseAndSyncNixyFile(appCtx, f)
	if err != nil {
		return nil, err
	}

	preloadHash := ""
	localFile := filepath.Join(appCtx.PWD, "nixy.local.yml")
	if !exists(localFile) {
		if appCtx.NixyPreload != nil {
			localFile = os.ExpandEnv(*appCtx.NixyPreload)
		}
	}

	if exists(localFile) {
		localNixy, err := parseAndSyncNixyFile(appCtx, localFile)
		if err != nil {
			return nil, err
		}
		preloadHash = localNixy.sha256Sum
	}

	nixy.projectHash = nw.sha256Sum
	nixy.localHash = preloadHash

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
