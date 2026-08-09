# home network
{ den, ... }:
{
  den.aspects.infra._.usergroups._.default = {
    aspect-name = "infra.usergroups.default";
    includes = [ den.aspects.infra._.providers._.unifi ];
    terranix = {
      resource.unifi_client_qos_rate.default = {
        name = "Default";
        site = "default";
      };

      import = [
        {
          to = "unifi_client_qos_rate.default";
          id = "6a20c6afaa20ab3db2a1669e";
        }
      ];
    };
  };
}
