NIXY_LAST_DIR=""

__nixy_debug() {
  echo "[## NIXY DEBUG] $@"
}

__nixy_shell_hook() {
  # If not in a nixy shell, and no nixy.yml, do nothing
  if [[ -z "$NIXY_SHELL" ]] && [[ ! -f nixy.yml ]]; then
    return
  fi

  # Set prompt prefix once, if inside NIXY shell
  if [[ -n "$NIXY_SHELL" ]] && [[ -z "$NIXY_PROMPT_PREFIX" ]]; then
    # Zsh uses %F and %f for color, no \[ \]
    local cyan dim_cyan reset
    cyan="%F{cyan}"
    dim_cyan="%F{cyan}%B"  # or a dim color; zsh doesn't have a native "dim" so adjust as desired
    reset="%f%b"

    NIXY_PROMPT_PREFIX="${NIXY_PROMPT_PREFIX:-${dim_cyan}[${cyan} ᵧ${dim_cyan}]${reset}}"
  fi

  # Auto-enter nixy shell when directory changes and not already in shell
  if [[ -z "$NIXY_SHELL" ]] && [[ "$NIXY_LAST_DIR" != "$PWD" ]]; then
    NIXY_LAST_DIR="$PWD"
    nixy shell
  fi
}

# Register hook if not already registered
if [[ -z "${precmd_functions[(r)__nixy_shell_hook]}" ]]; then
  precmd_functions+=(__nixy_shell_hook)
fi
