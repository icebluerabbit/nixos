# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.dmz = {
    aspect-name = "infra.ports.dmz";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.dmz = {
        name = "DMZ Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.dmz.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.dmz";
          id = "6a3049b2efa3052c88b3c531";
        }
      ];
    };
  };
}
