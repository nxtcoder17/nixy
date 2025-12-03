NIXY_LAST_DIR=""

__nixy_debug() {
  echo "[## NIXY DEBUG] $@"
}

__nixy_shell_hook() {
  if [[ -z "$NIXY_SHELL" ]] && [[ ! -f nixy.yml ]]; then
    __ps1_cleanup
    return
  fi

  if [[ -n "$NIXY_SHELL" ]] && [[ -z "$NIXY_PROMPT_PREFIX" ]]; then
    local cyan dim_cyan color_reset
    cyan="\[\033[0;36m\]"
    dim_cyan="\[\033[2;36m\]"
    color_reset="\[\033[0m\]"

    NIXY_PROMPT_PREFIX="${NIXY_PROMPT_PREFIX:-${dim_cyan}[${cyan} ᵧ${dim_cyan}]${color_reset}}"
  fi

  if [[ -z "$NIXY_SHELL" ]] && [[ "$NIXY_LAST_DIR" != "$PWD" ]]; then
    NIXY_LAST_DIR="$PWD"
    nixy shell
  fi
}

case "$PROMPT_COMMAND" in
  *__nixy_shell_hook*) ;;
  *)
    PROMPT_COMMAND="__nixy_shell_hook${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
    ;;
esac
