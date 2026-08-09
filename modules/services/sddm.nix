/*
  SDDM — Qt-based display manager for X11 and Wayland; used as the login manager with Wayland backend
  https://github.com/sddm/sddm
*/
_: {
  den.aspects.services._.sddm = {
    aspect-name = "services.sddm";
    nixos = {
      services.displayManager.sddm.enable = true;
    };
  };
}
