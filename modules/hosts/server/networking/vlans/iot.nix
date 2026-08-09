# iot — VLAN and bridge interface for iot subnet
{ den, ... }:
{
  den.aspects.hosts._.server._.networking._.vlans._.iot = {
    aspect-name = "hosts.server.networking.vlans.iot";
    includes = [
      den.aspects.hosts._.server._.networking._.bonds
      den.aspects.infra._.networks._.iot
      den.aspects.infra._.clients._.server-vlan-iot
      den.aspects.infra._.clients._.server-bridge-iot
    ];

    nixos =
      { config, pkgs, ... }:
      {
        networking = {
          vlans.vlan-iot = {
            id = config.infra.vlans.iot.vlanId;
            interface = "bond0";
          };
          bridges.br-iot.interfaces = [ "vlan-iot" ];
          interfaces = {
            vlan-iot.macAddress = config.infra.clients.server-vlan-iot.mac;
            br-iot = {
              macAddress = config.infra.clients.server-bridge-iot.mac;
              ipv4.addresses = [
                {
                  address = config.infra.clients.server-bridge-iot.ip;
                  prefixLength = config.infra.vlans.iot.subnetPrefix;
                }
              ];
            };
          };
        };

        # networking.bridges exposes no multicast-querier option; without an
        # active querier, the bridge's IGMP snooping silently stops flooding
        # multicast (mDNS/_hap etc.) to ports after the initial join times
        # out, breaking HomeKit/zeroconf discovery for containers on br-iot.
        systemd.services.br-iot-mcast-querier = {
          description = "Enable IGMP multicast querier on br-iot";
          after = [ "sys-subsystem-net-devices-br\\x2diot.device" ];
          bindsTo = [ "sys-subsystem-net-devices-br\\x2diot.device" ];
          wantedBy = [ "sys-subsystem-net-devices-br\\x2diot.device" ];
          serviceConfig.Type = "oneshot";
          script = "${pkgs.iproute2}/bin/ip link set dev br-iot type bridge mcast_querier 1";
        };
      };
  };
}
