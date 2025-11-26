NIXY_LAST_DIR=""

__nixy_shell_hook() {
  if ([[ "$NIXY_LAST_DIR" == "$PWD" ]] ||
      [[ -n "$NIXY_SHELL" ]] ||
      [[ ! -f nixy.yml ]]); then
    __ps1_cleanup
    return
  fi

  __ps1_add_prefix

  nixy shell
  NIXY_LAST_DIR="$PWD"
}

__ps1_add_prefix() {
  __nixy_premod_PS1="$PS1"

  local cyan dim_cyan color_reset
  cyan="\[\033[0;36m\]"
  dim_cyan="\[\033[2;36m\]"
  color_reset="\[\033[0m\]"

  NIXY_PROMPT_PREFIX="${NIXY_PROMPT_PREFIX:-${dim_cyan}[${cyan} ᵧ${dim_cyan}]${color_reset}}"
  PS1="${NIXY_PROMPT_PREFIX}$PS1"
}

__ps1_cleanup() {
  if [ -n "$__nixy_premod_PS1" ]; then
    PS1=$__nixy_premod_PS1
  fi
}

# Add or Append to PROMPT_COMMAND env var
PROMPT_COMMAND="__nixy_shell_hook${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
