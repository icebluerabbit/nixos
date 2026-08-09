# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.trust = {
    aspect-name = "infra.ports.trust";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.trust = {
        name = "Trust Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.trust.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.trust";
          id = "6a3049b2efa3052c88b3c52f";
        }
      ];
    };
  };
}
