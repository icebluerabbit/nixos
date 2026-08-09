# fonts — system-wide font configuration and fontconfig defaults
{ den, ... }:
{
  den.aspects.fonts._.fonts = {
    aspect-name = "fonts.fonts";
    includes = [
      den.aspects.fonts._.inter
      den.aspects.fonts._.jetbrains-mono
      den.aspects.fonts._.noto
      den.aspects.fonts._.noto-color-emoji
    ];

    provides.to-hosts.nixos = _: {
      fonts.fontconfig.defaultFonts = {
        sansSerif = [
          "Inter"
          "Noto Sans CJK KR"
          "Noto Sans CJK JP"
          "Noto Sans CJK SC"
        ];
        serif = [
          "Noto Serif"
          "Noto Serif CJK KR"
          "Noto Serif CJK JP"
          "Noto Serif CJK SC"
        ];
        monospace = [
          "JetBrainsMono Nerd Font"
          "Noto Sans Mono CJK KR"
          "Noto Sans Mono CJK JP"
          "Noto Sans Mono CJK SC"
        ];
        emoji = [
          "Noto Color Emoji"
        ];
      };
    };
  };
}
