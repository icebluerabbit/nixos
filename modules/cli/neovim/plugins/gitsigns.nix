{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.gitsigns = {
    aspect-name = "cli.neovim.plugins.gitsigns";
    includes = [ den.aspects.cli._.git ];

    homeManager = _: {
      programs.nixvim.plugins.gitsigns = {
        enable = true;
        lazyLoad = {
          enable = true;
          settings.event = [
            "BufReadPre"
            "BufReadPost"
            "BufNewFile"
            "BufEnter"
          ];
        };
        settings = {
          current_line_blame = false;
          current_line_blame_opts = {
            virt_text = true;
            virt_text_pos = "eol";
          };
          signcolumn = true;
          signs = {
            add.text = "▎";
            change.text = "▎";
            delete.text = "▁";
            topdelete.text = "▔";
            changedelete.text = "▎";
            untracked.text = "┋";
          };
          watch_gitdir = {
            follow_files = true;
          };
        };
      };
    };
  };
}
