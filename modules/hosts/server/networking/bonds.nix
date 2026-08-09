# bonds — link aggregation (LACP) bonding interface
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.bonds = {
    aspect-name = "hosts.server.networking.bonds";
    includes = [
      den.aspects.hosts._.server._.networking._.links
    ];

    nixos = _: {
      networking = {
        bonds.bond0 = {
          interfaces = [
            "enp5s0f0np0"
            "enp5s0f1np1"
          ];
          driverOptions = {
            mode = "802.3ad";
            xmit_hash_policy = "layer3+4";
            miimon = "100";
          };
        };

        interfaces.bond0.macAddress = "62:f5:a4:5b:20:b1";
      };
    };
  };
}
