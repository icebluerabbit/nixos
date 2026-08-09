# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.guest = {
    aspect-name = "infra.ports.guest";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.guest = {
        name = "Guest Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.guest.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.guest";
          id = "6a3049b2efa3052c88b3c52a";
        }
      ];
    };
  };
}
