# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.mgmt = {
    aspect-name = "infra.ports.mgmt";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.mgmt = {
        name = "MGMT Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.mgmt.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.mgmt";
          id = "6a2e2450efa3052c88b38fcb";
        }
      ];
    };
  };
}
