NIXY_LAST_DIR=""

__nixy_shell_hook() {
  if [[ -z "$NIXY_SHELL" ]] && [[ ! -f nixy.yml ]]; then
    __ps1_cleanup
    return
  fi

  if [[ -n "$NIXY_SHELL" ]]; then
    __ps1_add_prefix
  fi

  if [[ -z "$NIXY_SHELL" ]] && [[ "$NIXY_LAST_DIR" != "$PWD" ]]; then
    NIXY_LAST_DIR="$PWD"
    nixy shell
  fi
}

__ps1_add_prefix() {
  if [[ -z "$__nixy_premod_PS1" ]]; then
    __nixy_premod_PS1="$PS1"
  fi


  local cyan dim_cyan color_reset
  cyan="\[\033[0;36m\]"
  dim_cyan="\[\033[2;36m\]"
  color_reset="\[\033[0m\]"

  NIXY_PROMPT_PREFIX="${NIXY_PROMPT_PREFIX:-${dim_cyan}[${cyan} ᵧ${dim_cyan}]${color_reset} }"

  # prevent double prefix
  if [[ "$PS1" != "${NIXY_PROMPT_PREFIX}"* ]]; then
    PS1="${NIXY_PROMPT_PREFIX}${__nixy_premod_PS1}"
  fi
}

__ps1_cleanup() {
  if [ -n "$__nixy_premod_PS1" ]; then
    PS1=$__nixy_premod_PS1
  fi
}

case "$PROMPT_COMMAND" in
  *__nixy_shell_hook*) ;;
  *)
    PROMPT_COMMAND="__nixy_shell_hook${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
    ;;
esac
