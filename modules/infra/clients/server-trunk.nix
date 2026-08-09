# home network
{ den, ... }:
let
  name = "Server Trunk";
  mac = "62:f5:a4:5b:20:b1";
  ip = null;
in
{
  den.aspects.infra._.clients._.server-trunk = {
    aspect-name = "infra.clients.server-trunk";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os = _: {
      infra.clients.server-trunk = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.server_trunk = {
        inherit mac name;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.server_trunk";
          id = mac;
        }
      ];
    };
  };
}
