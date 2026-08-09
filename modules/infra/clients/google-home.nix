# home network
{ den, ... }:
let
  name = "Google Home";
  mac = "f4:f5:d8:b4:ff:78";
  ip = "10.10.30.188";
in
{
  den.aspects.infra._.clients._.google-home = {
    aspect-name = "infra.clients.google-home";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.google-home = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.google_home = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.google_home";
          id = mac;
        }
      ];
    };
  };
}
