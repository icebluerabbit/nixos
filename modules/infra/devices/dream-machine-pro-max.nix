# home network
{ den, lib, ... }:
let
  device = (import ../_mk-device.nix { inherit lib; }) {
    id = "dream-machine-pro-max";
    resourceName = "udm_pro_max";
    name = "Dream Machine Pro Max";
    mac = "74:fa:29:50:06:e5";
    # Uplink: USW Pro HD 24 PoE port 27.
    portProfile = "gateway-trunk";
    extra = {
      switch_vlan_enabled = false;
    };
  };
in
{
  den.aspects.infra._.devices._.dream-machine-pro-max = {
    aspect-name = "infra.devices.dream-machine-pro-max";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.ports._.gateway-trunk
    ];

    os = _: device.mkOs;
    terranix = { config, ... }: device.mkTerranix config.infra.ports;
  };
}
