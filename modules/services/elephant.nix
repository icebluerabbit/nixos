/*
  elephant — data provider backend daemon for the Walker launcher
  https://github.com/abenz1267/elephant
*/
_: {
  den.aspects.services._.elephant = {
    aspect-name = "services.elephant";
    nixos =
      { pkgs, ... }:
      {
        services.elephant.enable = true;
        systemd.user.services.elephant = {
          path = [
            pkgs.bash
            "/run/current-system/sw" # System-wide packages
            "/etc/profiles/per-user/%u" # User-specific packages (%u is systemd's variable for your username)
          ];
        };
      };
  };
}
