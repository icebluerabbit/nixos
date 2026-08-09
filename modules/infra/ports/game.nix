# home network
{ den, ... }:
{
  den.aspects.infra._.ports._.game = {
    aspect-name = "infra.ports.game";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_port_profile.game = {
        name = "Game Port";
        forward = "customize";
        op_mode = "switch";
        poe_mode = "auto";
        native_networkconf_id = "\${unifi_network.game.id}";
        site = "default";
      };

      import = [
        {
          to = "unifi_port_profile.game";
          id = "6a2e15faefa3052c88b38e13";
        }
      ];
    };
  };
}
