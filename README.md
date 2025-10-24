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
Download and install packages directly from URLs with environment variable expansion:
```yaml
packages:
  - name: kubectl
    url: https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl
    sha256: "abc123..."  # Optional, auto-fetched if not provided
    
  - name: terraform
    url: https://releases.hashicorp.com/terraform/1.6.0/terraform_1.6.0_linux_amd64.zip
    # Automatically detects archive type and extracts
    
  # Environment variables are expanded in URLs
  - name: my-tool
    url: https://github.com/org/tool/releases/download/v${MY_VERSION}/tool-${NIXY_OS}-${NIXY_ARCH}.tar.gz
    # Uses MY_VERSION from env section and built-in NIXY_* vars

env:
  MY_VERSION: "1.2.3"  # Custom env vars can be used in URLs
```

Built-in environment variables available:
- `NIXY_OS` - Operating system (e.g., linux, darwin)
- `NIXY_ARCH` - Architecture (e.g., amd64, arm64)
- `NIXY_ARCH_FULL` - Full architecture string (e.g., x86_64)

### 🔧 Build System
Define reproducible builds:
```yaml
builds:
  my-app:
    packages:
      - out/my-binary
      - config.json
    hook: |
      go build -o out/my-binary ./cmd/main.go
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
NIXY_EXECUTOR=local-ignore-env nixy shell
```

#### Docker
Perfect for CI/CD pipelines:
```bash
NIXY_EXECUTOR=docker nixy shell
```

#### Bubblewrap (Sandboxed)
Strong isolation with automatic static nix binary download - **no systemwide Nix installation required**:
```bash
NIXY_EXECUTOR=bubblewrap nixy shell
```

### 👤 Profile Management

> [!NOTE]
> Profiles provide file system isolation. Different dev shells within the same profile share some filesystems like nix-store, fake-home, etc.

Keep separate development profiles:
```bash
# Create profiles
nixy profile create work
nixy profile create personal

# Use a profile
NIXY_PROFILE=work nixy shell

# List profiles
nixy profile list
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
    readOnly: true
```

## Commands

### Core Commands
- `nixy init` - Initialize a new nixy.yml
- `nixy shell` - Enter development shell
- `nixy build [target]` - Build defined targets
- `nixy shell:hook <shell>` - Output shell hook script for auto-activation (supports: bash, zsh, fish)

### Profile Commands
- `nixy profile create <name>` - Create new profile
- `nixy profile list` - List all profiles
- `nixy profile remove <name>` - Remove profile

### Utility Commands
- `nixy validate` - Validate nixy.yml
- `nixy version` - Show version information

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
    url: https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl
  - name: helm
    url: https://get.helm.sh/helm-v3.13.0-linux-amd64.tar.gz
  - name: k9s
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
    url: <url>
    sha256: <hash>                    # Optional
    type: binary|archive              # Auto-detected

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
    readOnly: true                    # Optional, defaults to false

# Shell initialization
onShellEnter: |
  <bash commands>

# Build targets
builds:
  <target>:
    packages:
      - <package>
    paths:
      - <file-path-1>
      - <file-path-2>
```

## Environment Variables

- `NIXY_EXECUTOR` - Execution backend (local, local-ignore-env, docker, bubblewrap)
- `NIXY_PROFILE`  - Profile name to use

## Troubleshooting

### "No Nix installation found"
- Install Nix, or use `NIXY_EXECUTOR=bubblewrap` for automatic Nix download

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
