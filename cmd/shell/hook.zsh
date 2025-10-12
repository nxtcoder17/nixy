NIXY_LAST_DIR=""

__nixy_shell_activate() {
  [[ "$NIXY_LAST_DIR" == "$PWD" ]] && return

  [[ -n "$NIXY_SHELL" ]] && return

  [[ ! -f nixy.yml ]] && return

  # Save cursor position before displaying prompt
  tput sc

  # Display prompt
  if command -v gum &>/dev/null; then
    gum style \
      --border rounded \
      --border-foreground 212 \
      --align center \
      --width 60 \
      --margin "1" \
      --padding "1 2" \
      --bold \
      --foreground 147 \
      "🔧 nixy.yml detected" "" "Press $(gum style --foreground 212 --bold 'ENTER') to launch nixy shell" "$(gum style --foreground 241 'any other key to skip • auto-yes in 2s')"
  else
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  nixy.yml detected in this directory"
    echo "  Press ENTER to launch nixy shell"
    echo "  (any other key to skip • auto-yes in 2s)"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  fi

  # Read user input with 2 second timeout
  local key
  local exit_code
  read -t 2 -k 1 -s key
  exit_code=$?

  # Restore cursor position and clear to end of screen
  tput rc
  tput ed

  # Launch nixy shell on ENTER or timeout
  if [[ $exit_code -eq 0 && -z "$key" ]] || [[ $exit_code -gt 128 ]]; then
    nixy shell
  fi

  NIXY_LAST_DIR="$PWD"
}

# Hook into prompt using precmd
autoload -Uz add-zsh-hook
add-zsh-hook precmd __nixy_shell_activate
