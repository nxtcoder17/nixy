# Nixy

Simple, powerful Nix development environments without the complexity.

## Why Nixy?

Nixy bridges the gap between Nix's powerful reproducibility and developer-friendly simplicity. Write a simple YAML file, get a fully reproducible development environment - no Nix knowledge required.

## Quick Start

```bash
# Initialize a new project
nixy init

# Enter development shell
nixy shell
```

`nixy init` creates the project config:

- `nixy.yml` - shared project config, pinning `nixpkgs.default` to the current nixos-unstable commit

## Auto Jump into nixy shell

Automatically enter the nixy shell when you navigate to a directory with a `nixy.yml` file.

### Shell Integration

Add the following to your shell configuration file:

<details>
<summary>Fish Shell</summary>

Append to `~/.config/fish/config.fish`:
```fish
nixy shell:hook fish | source
```
</details>

<details>
<summary>Bash</summary>

Append to `~/.bashrc`:
```bash
eval "$(nixy shell:hook bash)"
```
</details>

<details>
<summary>Zsh</summary>

Append to `~/.zshrc`:
```zsh
eval "$(nixy shell:hook zsh)"
```
</details>

### Features
- Supports auto-entering the nixy shell when `nixy.yml` is in the current directory
- Shows interactive prompt with 2-second timeout
- Press ENTER to launch, any other key to skip
- Auto-launches after timeout if no input
- Prevents duplicate activation when already in nixy shell
- Optional fancy styling with [gum](https://github.com/charmbracelet/gum)

## Installation

### Pre-built Binary (Recommended)
Download from [releases page](https://github.com/nxtcoder17/nixy/releases)

### From Source
```bash
go install github.com/nxtcoder17/nixy/cmd@latest
```

### Using Nix
```bash
nix run github:nxtcoder17/nixy -- shell
```

### Docker Images
Both images are published multi-arch (`linux/amd64` + `linux/arm64`) to ghcr.io:

- `ghcr.io/nxtcoder17/nix` - nix runtime with flakes enabled. Built declaratively with `dockerTools.buildLayeredImage` from `packages.nix-image` in `flake.nix` — its contents are exactly the closure of the listed packages (`nix`, `bash`, `coreutils`, `curl` + ca-certs), with nothing bundled that isn't declared. Used as the builder base (`Dockerfile.build`) and as nixy's docker-mode image (`NIXY_DOCKER_MODE_IMAGE` default, tag `latest`; nixy itself is bind-mounted at `/bin/nixy` by the docker executor).
- `ghcr.io/nxtcoder17/nixy` - same nix runtime with the `nixy` binary baked in at `/usr/local/bin/nixy`.

```bash
docker run --rm -it ghcr.io/nxtcoder17/nixy nixy --version
docker run --rm -it ghcr.io/nxtcoder17/nix nix --version
```

Build the nix image locally with (nix required):
```bash
nix build .#nix-image && docker load < ./result
```

Build the nixy image locally with:
```bash
docker build -f Dockerfile.nixy --build-arg VERSION=<nixy-version> -t ghcr.io/nxtcoder17/nixy:<nixy-version> .
```

## Core Features

### 🎯 Simple YAML Configuration
No more complex Nix expressions. Just list what you need:

```yaml
# Define multiple nixpkgs versions (default is required)
nixpkgs:
  default: abc123
  unstable: def456

packages:
  - nodejs           # Uses default nixpkgs
  - python3
  - unstable#go      # Uses unstable nixpkgs

libraries:      # System libraries
  - zlib
  - openssl

onShellEnter: |    # Shell initialization
  echo "Welcome to your dev environment!"
```

### 📦 Advanced Package Management

#### Nixpkgs Packages
```yaml
# Define multiple nixpkgs versions
nixpkgs:
  default: dfb2f12e899db4876308eba6d93455ab7da304cd
  stable: abc123
  unstable: def456

packages:
  - nodejs                    # Latest from default nixpkgs
  - go_1_21                  # Specific version from default
  - stable#golang            # Package from stable nixpkgs
  - unstable#python314       # Package from unstable nixpkgs
```

#### URL Packages
Download and install packages directly from URLs:
```yaml
packages:
  - name: kubectl
    sources:
      linux/amd64:
        url: https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl
        sha256: "abc123..."  # Optional, auto-fetched if not provided
     
  - name: terraform
    sources:
      linux/amd64:
        url: https://releases.hashicorp.com/terraform/1.6.0/terraform_1.6.0_linux_amd64.zip
    # Automatically detects archive type and extracts
```

The source is picked by matching the `<os>/<arch>` where the shell runs: the
host in `local` mode, and the Linux container or VM in `docker`/`colima` mode.
Multiple platforms can be listed side by side.

The full archive is kept under `$out/package/` in the Nix store, so you
can inspect it later. Executable files found within three levels of the archive
root are symlinked into `$out/bin`, which is what lands on `PATH`.

To control exactly which executables are exposed, set `binaries`. Entries are
relative to the archive root and may be globs. A file entry is linked and made
executable; a directory entry contributes its executable children:
```yaml
  - name: mytool
    sources:
      linux/amd64:
        url: https://example.com/mytool-linux-amd64.tar.gz
    binaries:
      - ./bin/mytool
      - ./extras/*
```

`installHook` runs instead of the default install step, so auto-detection and
`binaries` do not apply when it is set.

### 🔧 Build System
Define reproducible runtime bundles that can be used outside `nixy shell` (for example in Docker images):
```yaml
builds:
  runtime:
    dir: .
    command: |
      go build -o ./bin/my-app ./cmd/my-app
    packages:
      - python314
      - curl
      - nginx
    paths:
      - bin/my-app
      - scripts/
      - config/
```

Run a target:
```bash
nixy build runtime
```

Generate a target-specific Dockerfile while building:
```bash
nixy build runtime --dockerfile
```

Output layout:
- `<dir>/.nixy/dist/app` - symlink to built output in `/nix/store`
- `<dir>/.nixy/dist/nix/store` - copied recursive store closure for that output

`dir` is optional and defaults to `.`. It controls where the generated Dockerfile and `.nixy/dist` directory are written.

This makes it easy to copy the runtime bundle into container images and run the binaries without entering a nix shell.

Docker usage pattern:
1. Run `nixy build <target> --dockerfile` in your project.
2. Build the image using the target's `dir` as the Docker build context.
3. Add `<copied-app>/bin` to `PATH`.

#### GitHub Actions (build + artifact)
```yaml
name: Build runtime bundle

on:
  push:
    branches: [main]

jobs:
  build-runtime:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: nxtcoder17/nixy@v1

      - name: Build target
        working-directory: example/docker-build
        run: nixy build runtime

      - name: Build Docker image from runtime bundle
        working-directory: example/docker-build
        run: |
          docker build \
            -f Dockerfile \
            -t nixy-runtime:ci \
            .

      - uses: actions/upload-artifact@v4
        with:
          name: nixy-runtime
          path: example/docker-build/.nixy/dist/
```

### 🏃 Multiple Execution Backends

#### Local (Default)
Uses your system's Nix installation and inherits host environment:
```bash
nixy shell
```

#### Local (Pure Environment)
Uses your system's Nix installation but creates a pure environment (ignores host environment variables):
```bash
NIXY_MODE=local-ignore-env nixy shell
```

#### Docker
Perfect for CI/CD pipelines:
```bash
NIXY_MODE=docker nixy shell
```

#### Bubblewrap (Sandboxed)
Strong isolation with automatic static nix binary download - **no systemwide Nix installation required**:
```bash
NIXY_MODE=bubblewrap nixy shell
```

> [!NOTE]
> Read more at [Enable kernel.unprivileged_userns_clone](https://wiki.archlinux.org/title/Podman#Enable_kernel.unprivileged_userns_clone)
> check output of `sysctl kernel.unprivileged_userns_clone`
> if it is not set to 1. Set it to 1, otherwise you won't be able to use bubblewrap
> ```sh
    echo "1" > /proc/sys/kernel/sysrq 
  ```

## Advanced Features

### 🌐 Mixed Package Sources
Combine packages from different nixpkgs versions:
```yaml
nixpkgs:
  default: dfb2f12e899db4876308eba6d93455ab7da304cd
  older: 9d1fa9fa266631335618373f8faad570df6f9ede

packages:
  - nodejs                    # From default nixpkgs
  - older#python311           # Python from older nixpkgs version
  - name: custom-tool
    sources:
      linux/amd64:
        url: https://example.com/tool.tar.gz
```

### 🗂️ Smart Archive Packages Handling
Automatically detects and extracts from various archive formats:
- `.tar`, `.tar.gz`, `.tar.xz`, `.tar.bz2`
- `.zip`, `.7z`, `.rar`
- `.gz`, `.xz`, `.bz2`

### 🔒 Pure Environments
All execution backends provide pure, reproducible environments with:
- Isolated dependencies
- Consistent behavior across machines
- No global state pollution

### ⚡ Intelligent Caching
- Configuration change detection via SHA256
- Rebuilds only when necessary
- Fast subsequent shells

### 🔎 Inspectable Generated Files
Nixy writes generated workspace files to the project-local `.nixy/` directory:

- `.nixy/flake.nix` - generated Nix flake used for the shell/build environment
- `.nixy/flake.lock` - generated lock file for the workspace flake
- `.nixy/nixy-merged.yml` - effective config used to generate the current workspace flake
- `.nixy/shell-enter.sh` - generated script from `onShellEnter`
- `.nixy/shell-env.sh` - generated environment from `nix print-dev-env`
- `.nixy/build.<target>.sh` - generated build script for a target (`/` and `:` are replaced with `_`)
- `.nixy/config.hash` - hash used to decide when generated files need refreshing

Build outputs remain under each build target directory, for example `<build.dir>/.nixy/dist`.
`.nixy/` should be gitignored because these files are regenerated from `nixy.yml`.

### 🎨 Environment Customization
```yaml
onShellEnter: |
  export EDITOR=vim
  alias ll='ls -la'
  echo "Environment ready!"

env:
  NODE_ENV: development
  DATABASE_URL: postgresql://localhost/myapp
  # Use $$ for literal dollar signs
  MY_VAR: "some-value-$$-with-dollar"
  # Reference other env vars (expands at runtime)
  PATH: "$PATH:/custom/bin"

# Mount additional directories (Docker/Bubblewrap only)
mounts:
  - source: /host/path
    dest: /container/path
    readonly: true
```

### 🌳 Git Worktrees

Non-local executors mount what the worktrees need, at their original absolute paths, so `git`
keeps working from any worktree inside the shell. A worktree's `.git` file points at the
shared git dir by absolute path, which is why the mount keeps the host layout instead of
aliasing it.

The common layout — the repo and its worktrees under a shared parent — needs a single mount.
That parent is skipped when it is unsafe to expose (`/`, your home directory). Worktrees that
share no safe parent with the repo, such as a bare repo whose worktrees live outside it, are
not supported yet. A repo with no worktrees, or no repo at all, mounts nothing.

This is automatic and needs no `mounts` entry. Worktrees created later appear as long as
they land inside an already mounted dir. Colima bakes mounts into the VM, so a worktree
outside the mounted dirs needs a profile restart (nixy warns once).

Set `NIXY_DISABLE_WORKTREE_MOUNTS=true` to opt out — for example when the sibling worktrees
hold things a sandboxed process shouldn't see. Only the worktree mount is dropped; explicit
`mounts:` entries are unaffected.

## Commands

### Core Commands
- `nixy init` - Initialize a new nixy.yml
- `nixy shell` - Enter development shell
- `nixy build [target]` - Build defined targets
- `nixy shell:hook <shell>` - Output shell hook script for auto-activation (supports: bash, zsh, fish)
- `nixy env` - List the environment variables nixy reads (add `--json` for tooling)

## Examples

### Full-Stack Development
```yaml
nixpkgs:
  default: dfb2f12e899db4876308eba6d93455ab7da304cd

packages:
  # Backend
  - nodejs
  - postgresql_15

  # Frontend
  - pnpm
  - cypress

  # Tools
  - name: stripe-cli
    sources:
      linux/amd64:
        url: https://github.com/stripe/stripe-cli/releases/download/v1.19.1/stripe_1.19.1_linux_x86_64.tar.gz

libraries:
  - postgresql

```

### Python Data Science
```yaml
nixpkgs:
  default: dfb2f12e899db4876308eba6d93455ab7da304cd
  cuda: abc123                                  # Specific version for CUDA

packages:
  - python311
  - poetry
  - jupyter
  - cuda#cudaPackages.cudatoolkit              # CUDA from cuda nixpkgs

libraries:
  - zlib
  - blas
  - lapack

env:
  JUPYTER_ENABLE_LAB: "true"
```

### Go with Kubernetes
```yaml
packages:
  - go_1_21
  - golangci-lint
  - gopls
  
  # Kubernetes tools
  - name: kubectl
    sources:
      linux/amd64:
        url: https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl
  - name: helm
    sources:
      linux/amd64:
        url: https://get.helm.sh/helm-v3.13.0-linux-amd64.tar.gz
  - name: k9s
    sources:
      linux/amd64:
        url: https://github.com/derailed/k9s/releases/download/v0.27.4/k9s_Linux_amd64.tar.gz

onShellEnter: |
  export KUBECONFIG=$HOME/.kube/config
```

## Configuration Reference

### nixy.yml
```yaml
# Define nixpkgs versions (default is required)
nixpkgs:
  default: <commit-hash>              # Required
  stable: <commit-hash>               # Optional
  unstable: <commit-hash>             # Optional

# Package list
packages:
  - <package-name>                    # Simple package (uses default)
  - <key>#<package>                   # From specific nixpkgs key
  - name: <name>                      # URL package
    sources:
      linux/amd64:
        url: <url>
        sha256: <hash>                # Optional
    binaries:                        # Optional, overrides executable auto-detection
      - <relative-path-or-glob>
    installHook: |                    # Optional
      <shell commands>

# System libraries
libraries:
  - <library-name>

# Environment variables
env:
  KEY: value                          # Static value
  PATH: "$PATH:/custom"               # Variable expansion
  ESCAPED: "value-$$-literal"         # Use $$ for literal $

# Additional mounts (Docker/Bubblewrap only)
mounts:
  - source: /host/path
    dest: /container/path
    readonly: true                    # Optional, defaults to false

# Shell initialization
onShellEnter: |
  <bash commands>

# Build targets
builds:
  <target>:
    dir: .                              # Optional Docker/output context directory, defaults to .
    command: |                          # Optional command run before packaging paths
      <shell commands>
    packages:                            # Optional nix packages in output
      - <package>
    paths:
      - <file-path-1>                    # Optional project paths to include
      - <file-path-2>
```

## Environment Variables

Run `nixy env` for the full list of variables nixy reads, with defaults and current values.
`nixy env --json` prints the same list for tooling.

Commonly used:

- `NIXY_MODE` - Execution backend (local, local-ignore-env, docker, bubblewrap, colima)
- `NIXY_DISABLE_WORKTREE_MOUNTS` - Set to `true` to keep sibling git worktrees out of non-local shells

## Troubleshooting

### "No Nix installation found"
- Install Nix, or use `NIXY_MODE=bubblewrap` for automatic Nix download

### "Package not found"
- Check package name at [search.nixos.org](https://search.nixos.org)
- Try updating nixpkgs commit hash

### "Archive extraction failed"
- Verify URL is accessible
- Check if SHA256 matches
- Ensure archive isn't corrupted

### Performance Issues
- Use local executor for fastest performance

## Contributing

We welcome contributions! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT

## Acknowledgments

Built with ❤️ using:
- [Nix](https://nixos.org/) - The powerful package manager
- [Go](https://golang.org/) - The programming language
- [bubblewrap](https://github.com/containers/bubblewrap) - Unprivileged Sandboxing tool

---

*Making Nix accessible to everyone, one YAML at a time.*
