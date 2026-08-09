/*
  JetBrains Mono — monospace typeface by JetBrains designed for code readability; patched as a Nerd Font for icon glyphs
  https://github.com/JetBrains/JetBrainsMono
*/
_: {
  den.aspects.fonts._.jetbrains-mono = {
    aspect-name = "fonts.jetbrains-mono";
    nixos =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.nerd-fonts.jetbrains-mono
          pkgs.jetbrains-mono
        ];
      };
  };
}
