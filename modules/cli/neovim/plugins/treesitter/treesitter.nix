{ den, ... }:
{
  den.aspects.cli._.neovim._.plugins._.treesitter = {
    aspect-name = "cli.neovim.plugins.treesitter";
    includes = [
      den.aspects.cli._.curl
      den.aspects.cli._.tar
      den.aspects.dev._.tree-sitter
    ];

    homeManager =
      { config, ... }:
      let
        grammars = config.programs.nixvim.plugins.treesitter.package.builtGrammars;
      in
      {
        programs.nixvim.plugins.treesitter = {
          enable = true;
          nixvimInjections = true;
          lazyLoad = {
            enable = true;
            settings.event = [
              "BufReadPre"
              "BufReadPost"
              "BufNewFile"
              "BufEnter"
            ];
          };
          settings.parser_install_dir = "${config.xdg.dataHome}/nvim/site";

          grammarPackages = with grammars; [
            bash
            go
            gomod
            gosum
            gowork
            json
            just
            lua
            make
            markdown
            nix
            regex
            rust
            toml
            vim
            vimdoc
            xml
            yaml
            zsh
          ];
        };
      };
  };
}
