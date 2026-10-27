/*
  JetBrains Mono — monospace typeface by JetBrains designed for code readability; patched as a Nerd Font for icon glyphs
  https://github.com/JetBrains/JetBrainsMono
*/
_: {
  den.aspects.fonts._.jetbrains-mono = {
    aspect-name = "fonts.jetbrains-mono";

    # `os` forwards into both nixos and darwin, and its route policy fires in
    # user scopes too — so reaching this from a user aspect (via style.stylix)
    # still lands in the host's system-level fonts.packages.
    os =
      { pkgs, ... }:
      {
        fonts.packages = [
          pkgs.nerd-fonts.jetbrains-mono
          pkgs.jetbrains-mono
        ];
      };
  };
}
