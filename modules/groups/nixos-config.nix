# nixos-config group — system group for NixOS configuration access
_: {
  den.aspects.groups._.nixos-config = {
    aspect-name = "groups.nixos-config";
    nixos.users.groups.nixos-config.gid = 900;
  };
}
