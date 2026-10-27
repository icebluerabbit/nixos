# home network
{ den, lib, ... }:
let
  device = (import ../_mk-device.nix { inherit lib; }) {
    id = "usw-flex-2-5g-8";
    resourceName = "switch_flex";
    name = "USW Flex 2.5G 8";
    mac = "94:2a:6f:4e:cc:ee";
    ip = "10.100.218.255";
    # Uplink: Flex port 9 into USW Pro HD 24 PoE port 9, both on this profile.
    portProfile = "living-room-trunk";

    ports = {
      "1".name = "MacBook Pro Work";
      "1".profile = "work-macbookpro-work-1";

      "2".name = "Hue Hub";
      "2".profile = "iot-hue-hub";

      "3".name = "PlayStation 5";
      "3".profile = "game-playstation5";

      "4".name = "Desktop";
      "4".profile = "trust-desktop";

      "9".name = "Uplink to USW Pro HD 24 PoE";
      "9".profile = "living-room-trunk";
    };

    extra = {
      stp_priority = 32768;
      stp_version = "rstp";
      switch_vlan_enabled = false;
    };
  };
in
{
  den.aspects.infra._.devices._.usw-flex-2-5g-8 = {
    aspect-name = "infra.devices.usw-flex-2-5g-8";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.ports._.game-playstation5
      den.aspects.infra._.ports._.iot-hue-hub
      den.aspects.infra._.ports._.living-room-trunk
      den.aspects.infra._.ports._.trust-desktop
      den.aspects.infra._.ports._.work-macbookpro-work-1
    ];

    os = _: device.mkOs;
    terranix = { config, ... }: device.mkTerranix config.infra.ports;
  };
}
