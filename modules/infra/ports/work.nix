# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.work = {
    aspect-name = "infra.ports.work";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.work = {
        name = "Work Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.work.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.work";
          id = "6a2e15faefa3052c88b38e11";
        }
      ];
    };
  };
}
