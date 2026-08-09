{ inputs, ... }:
{
  flake-file.inputs = {
    configuratarr = {
      url = "github:icebluerabbit/configuratarr/dev";
      inputs.nixpkgs.follows = "nixpkgs";
    };
  };

  den.aspects.services._.configuratarr = {
    aspect-name = "services.configuratarr";
    nixos = _: {
      imports = [ inputs.configuratarr.nixosModules.default ];
    };
  };
}
