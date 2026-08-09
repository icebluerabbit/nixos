# home network
{ den, ... }:
let
  name = "Hue Hub";
  mac = "00:17:88:79:81:e0";
  ip = "10.10.30.33";
in
{
  den.aspects.infra._.clients._.hue-hub = {
    aspect-name = "infra.clients.hue-hub";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.hue-hub = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.hue_hub = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.hue_hub";
          id = mac;
        }
      ];
    };
  };
}
