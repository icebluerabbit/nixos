# home network
{ den, ... }:
let
  name = "iPad";
  mac = "4e:37:93:b9:e0:6a";
  ip = "10.10.20.50";
in
{
  den.aspects.infra._.clients._.ipad = {
    aspect-name = "infra.clients.ipad";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.ipad = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.ipad = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.ipad";
          id = mac;
        }
      ];
    };
  };
}
