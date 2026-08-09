# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.apps = {
    aspect-name = "infra.ports.apps";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.apps = {
        name = "Apps Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.apps.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.apps";
          id = "6a3049b2efa3052c88b3c541";
        }
      ];
    };
  };
}
