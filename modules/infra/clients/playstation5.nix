# home network
{ den, ... }:
let
  name = "Sony PlayStation 5";
  mac = "b4:0a:d8:2b:e5:68";
  ip = "10.10.80.45";
in
{
  den.aspects.infra._.clients._.playstation5 = {
    aspect-name = "infra.clients.playstation5";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.playstation5 = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.playstation5 = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.playstation5";
          id = mac;
        }
      ];
    };
  };
}
