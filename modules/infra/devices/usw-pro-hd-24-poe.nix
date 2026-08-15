# home network
{ den, lib, ... }:
let
  device = (import ../_mk-device.nix { inherit lib; }) {
    id = "usw-pro-hd-24-poe";
    resourceName = "switch_pro";
    name = "USW Pro HD 24 PoE";
    mac = "a8:9c:6c:04:71:dc";
    ip = "10.100.103.119";

    ports = {
      "1".name = "U7 Pro XG AP";
      "1".profile = "ap-trunk";

      "2".name = "Server";
      "2".profile = "mgmt-server";

      "3".name = "Server IPMI";
      "3".profile = "mgmt-server-ipmi";

      "9".name = "USW Flex Uplink";
      "9".profile = "living-room-trunk";

      # The LACP pair into the server's bond0. Port 26 has no entry of its own —
      # it is claimed here, and the controller rejects configuring a member
      # directly.
      "25" = {
        name = "Server Trunk";
        profile = "server-trunk";
        aggregateMembers = [
          25
          26
        ];
        extra = {
          autoneg = true;
          egress_rate_limit_kbps_enabled = false;
          flow_control_enabled = false;
          full_duplex = true;
          isolation = false;
          lldpmed_enabled = true;
          lldpmed_notify_enabled = true;
          port_keepalive_enabled = true;
          port_security_enabled = false;
          stormctrl_bcast_enabled = false;
          stormctrl_mcast_enabled = false;
          stormctrl_ucast_enabled = false;
          stp_port_mode = true;
        };
      };

      "27".name = "Gateway Uplink";
      "27".profile = "gateway-trunk";

      "28".name = "UNAS Pro";
      "28".profile = "storage-unas-pro";
    };

    extra = {
      lcm_brightness = 100;
      lcm_night_mode_begins = "22:00";
      lcm_night_mode_ends = "08:00";
      stp_priority = 32768;
      stp_version = "rstp";
      switch_vlan_enabled = false;
    };
  };
in
{
  den.aspects.infra._.devices._.usw-pro-hd-24-poe = {
    aspect-name = "infra.devices.usw-pro-hd-24-poe";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
      den.aspects.infra._.ports._.ap-trunk
      den.aspects.infra._.ports._.gateway-trunk
      den.aspects.infra._.ports._.living-room-trunk
      den.aspects.infra._.ports._.mgmt-server
      den.aspects.infra._.ports._.mgmt-server-ipmi
      den.aspects.infra._.ports._.server-trunk
      den.aspects.infra._.ports._.storage-unas-pro
    ];

    os = _: device.mkOs;
    terranix = { config, ... }: device.mkTerranix config.infra.ports;
  };
}
