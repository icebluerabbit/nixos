/*
  udisks2 — disk management service for querying, mounting, and unmounting storage devices
  https://www.freedesktop.org/wiki/Software/udisks/
*/
_: {
  den.aspects.services._.udisks = {
    aspect-name = "services.udisks";
    nixos = {
      services.udisks2.enable = true;
    };
  };
}
