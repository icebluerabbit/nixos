# home network
{ den, ... }:
let
  name = "MacBookPro (work 1)";
  mac = "a0:ce:c8:a0:f1:27";
  ip = "10.10.70.243";
in
{
  den.aspects.infra._.clients._.macbookpro-work-1 = {
    aspect-name = "infra.clients.macbookpro-work-1";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.macbookpro-work-1 = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.macbookpro_work_1 = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.macbookpro_work_1";
          id = mac;
        }
      ];
    };
  };
}
