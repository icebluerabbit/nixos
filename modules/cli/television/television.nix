/*
  television — blazing-fast fuzzy finder TUI in Rust; channels-based architecture for files, git, env vars, and more
  https://github.com/alexpasmantier/television
*/
{ den, ... }:
{
  den.aspects.cli._.television = {
    aspect-name = "cli.television";
    includes = [
      den.aspects.cli._.television._.dirs
      den.aspects.cli._.television._.env
      den.aspects.cli._.television._.files
      den.aspects.cli._.television._.git-branch
      den.aspects.cli._.television._.git-diff
      den.aspects.cli._.television._.git-log
      den.aspects.cli._.television._.git-reflog
      den.aspects.cli._.television._.journal
      den.aspects.cli._.television._.just-recipes
      den.aspects.cli._.television._.mounts
      den.aspects.cli._.television._.ports
      den.aspects.cli._.television._.procs
      den.aspects.cli._.television._.systemd-units
      den.aspects.cli._.television._.tmux-sessions
      den.aspects.cli._.television._.tmux-windows
      den.aspects.cli._.television._.zoxide
      den.aspects.cli._.television._.zsh-history
    ];

    homeManager =
      { config, ... }:
      {
        programs.television = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          settings = {
            tick_rate = 50;
            ui = {
              use_nerd_font_icons = true;
              ui_scale = 100;
              layout = "landscape";
              input_bar_position = "top";

              preview_panel = {
                size = 50;
                scrollbar = true;
              };

              status_bar = {
                separator_open = "▐";
                separator_close = "▌";
              };

              features = {
                preview_panel = {
                  enabled = true;
                  visible = true;
                };
                remote_control = {
                  enabled = true;
                  visible = false;
                };
                help_panel = {
                  enabled = true;
                  visible = false;
                };
                status_bar = {
                  enabled = true;
                  visible = true;
                };
              };
            };
            keybindings = {
              # Navigation
              down = "select_next_entry";
              ctrl-j = "select_next_entry";
              up = "select_prev_entry";
              ctrl-k = "select_prev_entry";
              # Selection
              enter = "confirm_selection";
              ctrl-y = "copy_entry_to_clipboard";
              # Toggles
              ctrl-p = "toggle_preview";
              ctrl-r = "toggle_remote_control";
              "?" = "toggle_help";
            };
          };
        };

        programs.nix-search-tv = {
          enable = true;
          enableTelevisionIntegration = true;
        };
      };
  };
}
