# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.storage = {
    aspect-name = "infra.ports.storage";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.storage = {
        name = "Storage Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.storage.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.storage";
          id = "6a2e19a8efa3052c88b38ea3";
        }
      ];
    };
  };
}
