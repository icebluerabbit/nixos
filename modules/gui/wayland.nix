/*
  Wayland session bootstrap — XDG portals, SDDM Wayland backend, and Ozone/Electron env vars for native Wayland rendering
  https://wayland.freedesktop.org
*/
{ den, ... }:
{
  den.aspects.gui._.wayland = {
    aspect-name = "gui.wayland";
    includes = [
      den.aspects.services._.sddm
    ];

    nixos =
      { pkgs, ... }:
      {
        # System-wide Wayland prerequisites
        xdg.portal = {
          enable = true;
          extraPortals = [ pkgs.xdg-desktop-portal-gtk ];
          config.common.default = [ "gtk" ];
        };

        environment.sessionVariables.NIXOS_OZONE_WL = "1";

        services.displayManager.sddm.wayland.enable = true;
      };

    homeManager = _: {
      home.sessionVariables = {
        # Tell Electron apps to use Wayland natively
        ELECTRON_OZONE_PLATFORM_HINT = "wayland";
      };

      # Xwayland is handled by niri's built-in on-demand xwayland-satellite
      # integration (gui.niri sets programs.niri.settings.xwayland-satellite.path).
      # niri exports $DISPLAY for all session children and spawns the satellite
      # only when an X11 client connects — no eager systemd unit needed here.
    };
  };
}
