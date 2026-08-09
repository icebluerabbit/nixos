/*
  GNOME Keyring — daemon that stores passwords, keys, and certificates
  https://wiki.gnome.org/Projects/GnomeKeyring
*/
_: {
  den.aspects.services._.gnome-keyring = {
    aspect-name = "services.gnome-keyring";
    nixos = {
      services.gnome.gnome-keyring.enable = true;
    };
  };
}
