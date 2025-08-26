# Nixy

A developer-friendly CLI tool that simplifies Nix usage by providing an approachable interface to manage project-specific development environments.

## Features

- **Simple Configuration**: Manage Nix packages with a straightforward YAML file
- **Multiple Execution Modes**: Run Nix locally, in Docker, or with sandboxed BubbleWrap isolation
- **Profile Management**: Create isolated development environments with separate Nix stores
- **Dynamic Flake Generation**: Automatically generates Nix flakes from simple package lists
- **Cross-Platform**: Supports Linux and macOS (amd64/arm64)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/nxtcoder17/nixy
cd nixy

# Build with Go
go build -o nixy ./cmd/main.go

# Or use the Runfile (requires 'run' tool)
run build
```

### Pre-built Binaries

Download the appropriate binary for your platform from the releases page.

## Quick Start

1. **Initialize a new project**:
   ```bash
   nixy init
   ```
   This creates a `nixy.yml` file in your current directory.

2. **Install packages**:
   ```bash
   nixy install nixpkgs#nodejs nixpkgs#python3
   ```

3. **Enter development shell**:
   ```bash
   nixy shell
   ```

## Configuration

Nixy uses a simple YAML configuration file (`nixy.yml`):

```yaml
packages:
  - nixpkgs#nodejs
  - nixpkgs#python3
  - nixpkgs#go
  # Pin to specific nixpkgs version
  - nixpkgs/a3a3dda3bacf61e8a39258a0ed9c924eeca8e293#nodejs
```

## Execution Modes

Nixy supports three execution modes, selectable via the `NIXY_EXECUTOR` environment variable:

### Local (default)
```bash
NIXY_EXECUTOR=local nixy shell
```
Runs Nix commands directly on your system.

### Docker
```bash
NIXY_EXECUTOR=docker nixy shell
```
Runs Nix inside a Docker container for better isolation.

### BubbleWrap (experimental)
```bash
NIXY_EXECUTOR=bubblewrap nixy shell
```
Uses BubbleWrap for sandboxed execution with profile-based isolation:
- Separate home directories per profile
- Isolated Nix stores
- Profile-specific binary directories

## Profile Management

Create and manage isolated development profiles:

```bash
# List all profiles
nixy profile list

# Create a new profile
nixy profile add myproject

# Edit profile configuration
nixy profile edit myproject
```

When using BubbleWrap executor, set the active profile:
```bash
NIXY_PROFILE=myproject nixy shell
```

## Commands

### `nixy init`
Creates a new `nixy.yml` configuration file in the current directory.

### `nixy install <packages...>`
Installs one or more Nix packages and adds them to your configuration.

### `nixy shell`
Launches an interactive shell with all configured packages available.

### `nixy profile`
Manage development profiles:
- `list` (or `ls`) - List all profiles
- `add <name>` (or `new`, `create`) - Create a new profile
- `edit <name>` - Edit profile configuration

## Building from Source

### Prerequisites
- Go 1.24.4 or later
- Nix (for development environment)
- Docker (optional, for Docker executor)
- BubbleWrap (optional, for BubbleWrap executor)

### Build Commands

```bash
# Development build with auto-reload
run build:watch

# Build for current platform
run build

# Build for all platforms
run build:all

# Docker build
docker build -f Dockerfile.build -t nixy .
```

## How It Works

1. **Configuration Discovery**: Nixy searches up the directory tree for a `nixy.yml` file
2. **Flake Generation**: Dynamically generates Nix flakes based on your package list
3. **Environment Setup**: Prepares a development shell with all specified packages
4. **Path Management**: Automatically discovers and adds package binaries to PATH

## References

- [Using Nix without root](https://zameermanji.com/blog/2023/3/26/using-nix-without-root/)
- [Semver Nix Packages](https://lazamar.co.uk/nix-versions/?channel=nixpkgs-unstable&package=nodejs)

## License

[Add your license here]

## Contributing

[Add contribution guidelines here]
