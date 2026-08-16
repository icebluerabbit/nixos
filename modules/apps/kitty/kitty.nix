/*
  kitty — GPU-accelerated terminal emulator
  https://github.com/kovidgoyal/kitty
  Font, opacity and the base16 palette come from stylix's kitty target, so
  none of them appear here. No mux server: kitty has no detachable sessions.
*/
{ den, ... }:
{
  den.aspects.apps._.kitty = {
    aspect-name = "apps.kitty";
    includes = [
      den.aspects.fonts._.jetbrains-mono
      den.aspects.cli._.clipboard
    ];

    homeManager =
      { config, lib, ... }:
      {
        home.sessionVariables.TERMINAL = "kitty";

        # kitty loads this by a hardcoded path, config_dir/tab_bar.py, so it
        # cannot be referenced out of the store the way an option value can.
        xdg.configFile."kitty/tab_bar.py".source = ./tab_bar.py;

        programs.kitty = {
          enable = true;

          settings = {
            window_padding_width = 16;
            hide_window_decorations = "yes";
            confirm_os_window_close = 0;
            enable_audio_bell = "no";
            scrollback_lines = 10000;
            copy_on_select = "clipboard";
            disable_ligatures = "never";

            # Notify when a command that ran for 10s or more finishes in a
            # window that isn't focused. Rides on the shell integration
            # home-manager already enables.
            notify_on_cmd_finish = "unfocused 10.0";
            # ctrl+shift+h opens the scrollback here instead of in less.
            # This invocation is kitty's own for neovim 0.12+: it opens the
            # buffer as a terminal so the ANSI colours survive, and maps q
            # to quit without writing.
            scrollback_pager = "nvim --cmd 'set eventignore=FileType' +'nnoremap q ZQ' +'call nvim_open_term(0, {})' +'set nomodified nolist' +'$' -";
            # Defaults minus nothing, plus a prompt before pasting anything
            # large enough to be an accident.
            paste_actions = "quote-urls-at-prompt,confirm,confirm-if-large";
            # Trail only cursors that sat still for 300ms, so it follows
            # real jumps and not every redraw in a TUI.
            cursor_trail = 300;

            # ~120fps, as a frame interval in milliseconds.
            repaint_delay = 8;
            input_delay = 0;
            sync_to_monitor = "no";

            tab_bar_edge = "bottom";
            tab_bar_align = "center";
            tab_bar_min_tabs = 2;
            tab_bar_show_new_tab_button = "no";
            # Pills, drawn by tab_bar.py. No built-in style caps both ends
            # of a tab.
            tab_bar_style = "custom";
            # A ceiling, not a reservation: kitty still shrinks tabs to
            # share the window width. The default of 0 (unlimited) lets one
            # long title crowd out every other tab.
            tab_title_max_length = 32;
            # Templates are compiled as f-strings, so this is real Python
            # over a fixed namespace. Strips the status glyph Claude Code
            # prefixes its title with, and the -wrapped suffix Nix binaries
            # carry: without them a tab reads as ".claude-unwrapped", which
            # is identical across every session. Padding is drawn by
            # tab_bar.py, inside the caps.
            tab_title_template = ''"{index} {re.sub("-u?n?wrapped$", "", re.sub("^[^0-9A-Za-z]+", "", title)) or "kitty"}"'';

            # Claude Code tints its own tab through the remote control
            # socket; see hooks/kitty-tab.sh. socket-only keeps the control
            # channel off the terminal's own escape-code stream.
            allow_remote_control = "socket-only";
            listen_on = "unix:@kitty-{kitty_pid}";

            # How long a ctrl+a chord stays open.
            map_timeout = "2.0";
          };

          keybindings = {
            "ctrl+a>c" = "new_tab";
            "ctrl+a>x" = "close_window_with_confirmation";
            # Replaces quick_select_patterns: the hints kitten picks URLs
            # out of the visible screen and opens them.
            "ctrl+a>u" = "kitten hints --type url --program default";
          }
          //
            # Tabs 1-9 plus 0 for the tenth. kitty's goto_tab is 1-based.
            lib.listToAttrs (
              lib.genList (
                i: lib.nameValuePair "ctrl+a>${toString (lib.mod (i + 1) 10)}" "goto_tab ${toString (i + 1)}"
              ) 10
            );

          # stylix's palette arrives as an `include` appended to this same
          # option, and kitty lets the last directive win, so overriding its
          # colours has to land after it — settings would be shadowed.
          #
          # Three pill colours that cannot collide: mauve for the active
          # tab, grey for inactive, and the four the Claude Code hook sets
          # per-tab (yellow, red, blue, green). Text on a coloured pill is
          # the terminal background; text on the grey one is normal
          # foreground.
          extraConfig = lib.mkAfter ''
            tab_bar_background none
            active_tab_background ${config.lib.stylix.colors.withHashtag.base0E}
            active_tab_foreground ${config.lib.stylix.colors.withHashtag.base00}
            inactive_tab_background ${config.lib.stylix.colors.withHashtag.base02}
            inactive_tab_foreground ${config.lib.stylix.colors.withHashtag.base05}
          '';
        };
      };
  };
}
