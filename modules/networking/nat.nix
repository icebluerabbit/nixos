# nat — NAT (Network Address Translation) configuration
_: {
  den.aspects.networking._.nat = {
    aspect-name = "networking.nat";
    nixos = _: {
      networking.nat.enable = true;
    };
  };
}
