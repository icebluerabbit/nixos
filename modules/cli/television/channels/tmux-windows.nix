{ den, ... }:
{
  den.aspects.cli._.television._.tmux-windows = {
    aspect-name = "cli.television.tmux-windows";
    includes = [ den.aspects.cli._.tmux ];

    homeManager =
      { pkgs, lib, ... }:
      let
        tmux = lib.getExe pkgs.tmux;
      in
      {
        programs.television.channels.tmux-windows = {
          metadata = {
            name = "tmux-windows";
            description = "List and switch between tmux windows";
          };
          source = {
            command = "${tmux} list-windows -a -F '#{session_name}:#{window_index}\t#{window_name}\t#{pane_current_command}'";
            display = "{split:\t:0} - {split:\t:1} ({split:\t:2})";
            output = "{split:\t:0}";
          };
          preview = {
            command = "${tmux} capture-pane -t '{split:\t:0}' -p 2>/dev/null || echo 'No preview available'";
          };
          actions = {
            select = {
              description = "Switch to the selected window";
              command = "${tmux} select-window -t '{split:\t:0}'";
              mode = "execute";
            };
            kill = {
              description = "Kill the selected window";
              command = "${tmux} kill-window -t '{split:\t:0}'";
              mode = "fork";
            };
          };
        };
      };
  };
}
