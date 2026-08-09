/*
  yazi — async terminal file manager in Rust; Lua plugin system, built-in previewer for images/video/archives
  https://github.com/sxyazi/yazi
*/
{ den, ... }:
{
  den.aspects.cli._.yazi = {
    aspect-name = "cli.yazi";
    includes = [ den.aspects.cli._.clipboard ];

    homeManager =
      { config, pkgs, ... }:
      {
        programs.yazi = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          shellWrapperName = "y";
          initLua = ./init.lua;
          settings = {
            mgr = {
              show_symlink = true;
              show_hidden = true;
            };
            keymap.mgr.prepend_keymap = [
              {
                on = "<C-y>";
                run = [ "plugin wl-clipboard" ];
                desc = "Copy file (or contents) to clipboard";
              }
              {
                on = "<C-d>";
                run = [ "plugin diff" ];
                desc = "Diff the selected with the hovered file";
              }
              {
                on = [
                  "c"
                  "m"
                ];
                run = [ "plugin chmod" ];
                desc = "Chmod on selected files";
              }
            ];
            plugin = {
              prepend_previewers = [
                {
                  url = "*.md";
                  run = "rich-preview";
                }
                {
                  url = "*.json";
                  run = "rich-preview";
                }
              ];
              prepend_fetchers = [
                {
                  id = "git";
                  url = "*";
                  run = "git";
                }
                {
                  id = "git";
                  url = "*/";
                  run = "git";
                }
              ];
            };
          };
          plugins = {
            inherit (pkgs.yaziPlugins)
              git
              diff
              chmod
              compress
              rich-preview
              wl-clipboard
              ;
          };
        };
      };
  };
}
