# home network
{ den, ... }:
let
  name = "MacBookPro (work, wifi)";
  mac = "ee:17:4a:7c:e1:18";
  ip = "10.10.70.42";
in
{
  den.aspects.infra._.clients._.macbookpro-work-2 = {
    aspect-name = "infra.clients.macbookpro-work-2";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.macbookpro-work-2 = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.macbookpro_work_2 = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.macbookpro_work_2";
          id = mac;
        }
      ];
    };
  };
}
