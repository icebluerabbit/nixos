# home network
{ den, ... }:
let
  name = "MacBookPro (personal)";
  mac = "66:87:b9:92:9c:77";
  ip = "10.10.20.86";
in
{
  den.aspects.infra._.clients._.macbookpro-personal = {
    aspect-name = "infra.clients.macbookpro-personal";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.macbookpro-personal = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.macbookpro_personal = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.macbookpro_personal";
          id = mac;
        }
      ];
    };
  };
}
