set -l last_dir ""

function __nixy_shell_activate --on-variable PWD
  test "$last_dir" = "$PWD" && return

  if not status is-interactive
    return
  end

  if test -n "$NIXY_SHELL"
    return
  end

  if not test -e nixy.yml
    return
  end

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

    # # Clear the banner
    # for i in (seq 1 8)
    #   tput cuu1
    #   tput el
    # end
  else
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  nixy.yml detected in this directory"
    echo "  Press ENTER to launch nixy shell (any other key to skip)..."
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # # Clear the prompt lines
    # for i in (seq 1 5)
    #   tput cuu1
    #   tput el
    # end
  end

  set -l response (bash -c 'read -t 2 -n 1 -s key; echo $?; echo $key')
  set -l exit_code $response[1]
  set -l key (string join '' $response[2..-1])

  if test $exit_code -eq 0 -a -z "$key"
    clear
    nixy shell
  else if test $exit_code -gt 128
    # Timeout occurred
    clear
    nixy shell
  end
  set last_dir "$PWD"
end

if status is-interactive
  __nixy_shell_activate
end

