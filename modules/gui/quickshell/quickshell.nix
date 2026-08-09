/*
  QuickShell — QtQuick/QML desktop shell toolkit for bars, widgets, and overlays; supports live reload and Wayland layer-shell
  https://git.outfoxxed.me/quickshell/quickshell
*/
{ inputs, den, ... }:
{
  flake-file.inputs = {
    quickshell = {
      url = "git+https://git.outfoxxed.me/outfoxxed/quickshell";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    qml-niri = {
      url = "github:imiric/qml-niri/main";
      inputs = {
        nixpkgs.follows = "nixpkgs";
        quickshell.follows = "quickshell";
        flake-parts.follows = "flake-parts";
      };
    };
  };

  den.aspects.gui._.quickshell = {
    aspect-name = "gui.quickshell";
    includes = [
      den.aspects.gui._.wayland
      den.aspects.hw._.graphics
      den.aspects.hw._.headset
      den.aspects.hw._.liquidctl
      den.aspects.cli._.jq
      den.aspects.services._.upower
    ];

    homeManager =
      { pkgs, config, ... }:
      let
        system = pkgs.stdenv.hostPlatform.system;
      in
      {
        xdg.configFile."ZMS/MyShell.conf".text = ''
          [Colors]
          base00=${config.lib.stylix.colors.withHashtag.base00}
          base01=${config.lib.stylix.colors.withHashtag.base01}
          base02=${config.lib.stylix.colors.withHashtag.base02}
          base03=${config.lib.stylix.colors.withHashtag.base03}
          base04=${config.lib.stylix.colors.withHashtag.base04}
          base05=${config.lib.stylix.colors.withHashtag.base05}
          base06=${config.lib.stylix.colors.withHashtag.base06}
          base07=${config.lib.stylix.colors.withHashtag.base07}
          base08=${config.lib.stylix.colors.withHashtag.base08}
          base09=${config.lib.stylix.colors.withHashtag.base09}
          base0A=${config.lib.stylix.colors.withHashtag.base0A}
          base0B=${config.lib.stylix.colors.withHashtag.base0B}
          base0C=${config.lib.stylix.colors.withHashtag.base0C}
          base0D=${config.lib.stylix.colors.withHashtag.base0D}
          base0E=${config.lib.stylix.colors.withHashtag.base0E}
          base0F=${config.lib.stylix.colors.withHashtag.base0F}

          [Fonts]
          monospace=${config.stylix.fonts.monospace.name}
          sansSerif=${config.stylix.fonts.sansSerif.name}
          serif=${config.stylix.fonts.serif.name}
          emoji=${config.stylix.fonts.emoji.name}

          [Shape]
          radiusSm=6
          radiusMd=10
          radiusLg=12
          radiusFull=999

          [Sizes]
          height=30
          marginXs=5
          marginSm=10
          marginMd=15
          marginLg=25
          spacingXs=5
          spacingSm=10
          spacingMd=15
          spacingLg=25
          iconSize=16
          textSize=13
        '';

        programs.quickshell = {
          enable = true;
          package = inputs.qml-niri.packages.${system}.quickshell.override {
            withWayland = true;
            withPipewire = true;
            withQtSvg = true;
          };
          systemd = {
            enable = true;
          };
          configs.default = ./default;
          activeConfig = "default";
        };
      };
  };
}
