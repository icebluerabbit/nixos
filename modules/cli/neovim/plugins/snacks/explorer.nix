{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.snacks._.explorer = {
    aspect-name = "cli.neovim.plugins.snacks.explorer";
    includes = [
      den.aspects.dev._.sqlite
      den.aspects.cli._.neovim._.plugins._.snacks._.picker
    ];

    homeManager = _: {
      programs.nixvim = {
        plugins.snacks.settings = {
          explorer = {
            enabled = true;
            replace_netrw = true;
          };
          picker.sources.explorer = {
            hidden = true;
          };
        };
        keymaps = [
          {
            key = "<leader>e";
            action = "<cmd>lua Snacks.explorer()<cr>";
            options = {
              desc = "File Explorer";
            };
          }
        ];
      };
    };
  };
}
