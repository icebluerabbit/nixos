# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.services = {
    aspect-name = "infra.ports.services";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.services = {
        name = "Services Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.services.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.services";
          id = "6a3049b2efa3052c88b3c52c";
        }
      ];
    };
  };
}
