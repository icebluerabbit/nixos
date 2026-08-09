# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.download = {
    aspect-name = "infra.ports.download";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.download = {
        name = "Download Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.download.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.download";
          id = "6a3049b2efa3052c88b3c52e";
        }
      ];
    };
  };
}
