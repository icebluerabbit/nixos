# home network
{ den, ... }:
let
  name = "iPhone";
  mac = "d6:c8:14:8c:ab:3e";
  ip = "10.10.20.60";
in
{
  den.aspects.infra._.clients._.iphone = {
    aspect-name = "infra.clients.iphone";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.iphone = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.iphone = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.iphone";
          id = mac;
        }
      ];
    };
  };
}
