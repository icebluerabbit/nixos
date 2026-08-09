# neovim: user-commands — custom Ex commands defined via vim.api.nvim_create_user_command
_: {
  den.aspects.cli._.neovim._.user-commands = {
    aspect-name = "cli.neovim.user-commands";
    homeManager = _: {
      programs.nixvim.userCommands = { };
    };
  };
}
