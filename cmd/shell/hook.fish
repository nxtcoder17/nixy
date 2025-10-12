set -g last_dir ""
function __nixy_shell_activate --on-variable PWD --on-event fish_prompt
  test "$last_dir" = "$PWD" && return

  if test -n "$NIXY_SHELL"
    return
  end

  if not test -e nixy.yml
    return
  end

  # Save cursor position before displaying prompt
  tput sc

  # Display prompt
  if type -q gum
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
  end

  # Read user input with 2 second timeout
  set -l response (bash -c "read -t 2 -n 1 -s key; echo \$?; echo \$key")
  set -l exit_code $response[1]
  set -l key (string join '' $response[2..-1])

  # Restore cursor position and clear to end of screen
  tput rc
  tput ed

  # Launch nixy shell on ENTER or timeout
  if test $exit_code -eq 0 -a -z "$key"; or test $exit_code -gt 128
    nixy shell
  end
  set -g last_dir "$PWD"
end

