# home network
{ den, ... }:
let
  name = "BroadLink Remote";
  mac = "34:8e:89:2d:c1:19";
  ip = "10.10.30.76";
in
{
  den.aspects.infra._.clients._.broadlink-remote = {
    aspect-name = "infra.clients.broadlink-remote";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.broadlink-remote = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.broadlink_remote = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.broadlink_remote";
          id = mac;
        }
      ];
    };
  };
}
