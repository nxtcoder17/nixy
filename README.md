# Nixy

Simple, powerful Nix development environments without the complexity.

## Quick Start

```bash
# Initialize a new project
nixy init

# Enter development shell
nixy shell
```

## Installation

### Pre-built Binary (Recommended)
Download from [releases page](https://github.com/nxtcoder17/nixy/releases)

### From Source
```bash
go install github.com/nxtcoder17/nixy/cmd@latest
```

## Configuration

Create a `nixy.yml`:

```yaml
# Optional: pin nixpkgs version
nixpkgs: abc123

packages:
  - nodejs
  - python3
  - go

libraries:      # Optional: system libraries
  - zlib
  - openssl

shellHook: |    # Optional: shell initialization
  echo "Welcome to your dev environment!"
```

## Features

- **Zero Nix Knowledge Required**: Just list packages you need
- **Multiple Backends**: Run locally, in Docker, or sandboxed (bubblewrap)
- **Profile Isolation**: Keep separate profiles
- **Works Without Nix**: Automatically downloads Nix if needed (works only with bubblewrap)

## Commands

- `nixy init` - Create a new nixy.yml
- `nixy shell` - Enter development shell
- `nixy profile add <name>` - Create isolated profile
- `nixy profile list` - List all profiles

## Execution Modes

```bash
# Local (default)
nixy shell

# Docker (requires Docker)
NIXY_EXECUTOR=docker nixy shell

# Sandboxed (requires [bubblewrap](https://github.com/containers/bubblewrap))
NIXY_EXECUTOR=bubblewrap nixy shell
```

## Examples

### Node.js Project
```yaml
# replace with current nixpkgs nightly
nixpkgs: dfb2f12e899db4876308eba6d93455ab7da304cd

packages:
  - nodejs
  - pnpm
```

### Python Development
```yaml
# replace with current nixpkgs nightly
nixpkgs: dfb2f12e899db4876308eba6d93455ab7da304cd

packages:
  - python311
  - poetry
libraries:
  - zlib
```

### Go with specific tools
```yaml
# replace with current nixpkgs nightly
nixpkgs: dfb2f12e899db4876308eba6d93455ab7da304cd

packages:
  - go_1_21
  - golangci-lint
  - gopls
```

## License

MIT

## Contributing

PRs welcome! See [CONTRIBUTING.md](CONTRIBUTING.md)
