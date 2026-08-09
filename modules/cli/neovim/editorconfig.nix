# neovim: editorconfig — built-in EditorConfig support and per-property overrides
_: {
  den.aspects.cli._.neovim._.editorconfig = {
    aspect-name = "cli.neovim.editorconfig";
    homeManager = _: {
      programs.nixvim.editorconfig = {
        enable = true;
        properties = { };
      };
    };
  };
}
