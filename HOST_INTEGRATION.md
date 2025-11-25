# Host Integration Guide

This guide explains how to expose host system resources to sandboxed nixy shells when using **bubblewrap** or **docker** mode. This includes GUI applications, timezone, container runtimes, and more.

## Where to Configure

Host integrations can be configured at two levels:

| Level | File | Use Case |
|-------|------|----------|
| **Profile** | `nixy profile edit` | Personal tools shared across all projects |
| **Project** | `nixy.yml` in project root | Project-specific needs for all contributors |

> **Tip**: Use profile-level config for your personal workflow (browser, neovim, container tools, etc.) to avoid cluttering project configs. Use project-level for things the whole team needs.

---

## GUI Applications

### Display Server (X11/Wayland)

The `/run` directory contains the sockets needed for GUI applications to render windows:
- Wayland socket (`$XDG_RUNTIME_DIR/wayland-0`)
- X11 socket (`/tmp/.X11-unix` is often symlinked here)
- PulseAudio/PipeWire sockets for audio

```yaml
mounts:
  - source: "/run"
    dest: "/run"
    readonly: true
```

### GPU Access

Direct Rendering Infrastructure (DRI) is required for hardware-accelerated graphics (OpenGL, Vulkan). Without this mount, apps fall back to software rendering which is slow and CPU-intensive.

```yaml
mounts:
  - source: "/dev/dri"
    dest: "/dev/dri"
    readonly: true
```

### Shared Memory

POSIX shared memory is required for Chrome, Electron apps, Firefox, and any multi-process GUI application. These apps use shared memory for IPC between renderer processes.

> **Note**: This mount should NOT be read-only.

```yaml
mounts:
  - source: "/dev/shm"
    dest: "/dev/shm"
```

### Fonts

User fonts and fontconfig cache are needed for proper text rendering in GUI apps.

```yaml
mounts:
  - source: "$XDG_DATA_HOME/fonts"
    dest: "$XDG_DATA_HOME/fonts"
    readonly: true

  - source: "$XDG_CACHE_HOME/fontconfig"
    dest: "$XDG_CACHE_HOME/fontconfig"
    readonly: true
```

### Complete Snippet

```yaml
mounts:
  - source: "/run"
    dest: "/run"
    readonly: true

  - source: "/dev/dri"
    dest: "/dev/dri"
    readonly: true

  - source: "/dev/shm"
    dest: "/dev/shm"

  - source: "$XDG_DATA_HOME/fonts"
    dest: "$XDG_DATA_HOME/fonts"
    readonly: true

  - source: "$XDG_CACHE_HOME/fontconfig"
    dest: "$XDG_CACHE_HOME/fontconfig"
    readonly: true

env:
  DISPLAY: "$DISPLAY"
  WAYLAND_DISPLAY: "$WAYLAND_DISPLAY"
  XDG_BACKEND: "$XDG_BACKEND"
  XDG_RUNTIME_DIR: "$XDG_RUNTIME_DIR"
  BROWSER: "google-chrome-stable"

packages:
  - xdg-utils
  - google-chrome
```

---

## Timezone

The timezone database is required for correct time display, date formatting, and scheduling.

> **Note**: `/etc/localtime` (symlink to current timezone) is already available since nixy mounts `/etc` readonly by default.

```yaml
mounts:
  - source: "/usr/share/zoneinfo"
    dest: "/usr/share/zoneinfo"
    readonly: true
```

---

## Container Runtimes

For running containers (Podman/Docker) from within the nixy shell.

### Podman Socket

The Podman socket allows running containers via the podman or docker CLI from within the sandboxed shell.

```yaml
mounts:
  - source: "$XDG_RUNTIME_DIR/podman/podman.sock"
    dest: "$XDG_RUNTIME_DIR/podman/podman.sock"
```

### Registry Credentials

Docker/registry credentials are needed for pulling images from private registries.

```yaml
mounts:
  - source: "$HOME/.config/docker/config.json"
    dest: "$HOME/.config/docker/config.json"
    readonly: true
```

### Complete Snippet

```yaml
mounts:
  - source: "$XDG_RUNTIME_DIR/podman/podman.sock"
    dest: "$XDG_RUNTIME_DIR/podman/podman.sock"

  - source: "$HOME/.config/docker/config.json"
    dest: "$HOME/.config/docker/config.json"
    readonly: true

env:
  DOCKER_HOST: "unix://$XDG_RUNTIME_DIR/podman/podman.sock"
  CONTAINER_HOST: "unix://$XDG_RUNTIME_DIR/podman/podman.sock"

packages:
  - podman
  - nerdctl
```

---

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| "cannot open display" | Missing DISPLAY env var or /run mount | Add both DISPLAY env and /run mount |
| Blank/black window | Missing GPU access | Add /dev/dri mount |
| Chrome crashes on startup | Missing /dev/shm or it's read-only | Add /dev/shm mount without readonly |
| Garbled/missing fonts | Missing font mounts | Add font directory mounts |
| No audio | Missing audio socket access | Ensure /run is mounted (contains PipeWire/PulseAudio sockets) |
| Wrong timezone | Missing zoneinfo mount | Add /usr/share/zoneinfo mount |
| "Cannot connect to Docker daemon" | Missing socket mount or env var | Add podman socket mount and DOCKER_HOST env |
| "unauthorized" when pulling images | Missing docker config | Add ~/.config/docker/config.json mount |
