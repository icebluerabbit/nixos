# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.iot = {
    aspect-name = "infra.ports.iot";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.iot = {
        name = "IoT Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.iot.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.iot";
          id = "6a2e15faefa3052c88b38e17";
        }
      ];
    };
  };
}
