_: {
  den.aspects.cli._.neovim._.plugins._.snacks._.notifier = {
    aspect-name = "cli.neovim.plugins.snacks.notifier";
    homeManager = _: {
      programs.nixvim = {
        plugins.snacks.settings.notifier = {
          enabled = true;
          timeout = 3000;
          width = {
            min = 40;
            max = 0.4;
          };
          height = {
            min = 1;
            max = 0.6;
          };
          margin = {
            top = 0;
            right = 1;
            bottom = 0;
          };
          padding = true;
          gap = 0;
          sort = [
            "level"
            "added"
          ];
          icons = {
            error = " ";
            warn = " ";
            info = " ";
            debug = " ";
            trace = " ";
          };
          style = "compact";
          top_down = true;
          date_format = "%R";
          more_format = " ↓ %d lines ";
          refresh = 50;
        };
        keymaps = [
          {
            key = "<leader>un";
            action = "<cmd>lua Snacks.notifier.hide()<cr>";
            options = {
              desc = "Dismiss All Notifications";
            };
          }
        ];
        autoCmd = [
          {
            callback.__raw = builtins.readFile ./notifier-lsp.lua;
            event = "LspProgress";
          }
        ];
      };
    };
  };
}
