/*
  Stylix — NixOS/Darwin/home-manager theming layer; applies a Base16 color scheme and font choices system-wide
  https://github.com/nix-community/stylix
*/
{ den, inputs, ... }:
let
  # Shared stylix settings, identical across NixOS and nix-darwin hosts;
  # only the imported platform module differs.
  stylixSettings = pkgs: {
    enable = true;
    autoEnable = true;
    enableReleaseChecks = false;
    base16Scheme = "${pkgs.base16-schemes}/share/themes/catppuccin-mocha.yaml";
    polarity = "dark";

    homeManagerIntegration.autoImport = true;
    homeManagerIntegration.followSystem = true;

    fonts = {
      monospace.name = "JetBrainsMono Nerd Font";
      sansSerif.name = "Inter";
      serif.name = "Noto Serif";
      emoji.name = "Noto Color Emoji";
    };
  };
in
{
  flake-file.inputs.stylix = {
    url = "github:nix-community/stylix";
    inputs.nixpkgs.follows = "nixpkgs";
  };

  den.aspects.style._.stylix = {
    aspect-name = "style.stylix";
    includes = [
      den.aspects.style._.base16
      den.aspects.fonts._.inter
      den.aspects.fonts._.jetbrains-mono
      den.aspects.fonts._.noto
      den.aspects.fonts._.noto-color-emoji
    ];

    provides.to-hosts.nixos =
      { pkgs, ... }:
      {
        imports = [ inputs.stylix.nixosModules.stylix ];
        stylix = stylixSettings pkgs;
      };

    provides.to-hosts.darwin =
      { pkgs, ... }:
      {
        imports = [ inputs.stylix.darwinModules.stylix ];
        stylix = stylixSettings pkgs;
      };
  };
}
