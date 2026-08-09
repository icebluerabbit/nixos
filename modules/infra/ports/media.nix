# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.media = {
    aspect-name = "infra.ports.media";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.media = {
        name = "Media Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.media.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.media";
          id = "6a3049b2efa3052c88b3c52d";
        }
      ];
    };
  };
}
