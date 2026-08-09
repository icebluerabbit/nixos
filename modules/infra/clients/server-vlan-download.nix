# home network
{ den, ... }:
let
  name = "Server Download";
  mac = "62:f5:a4:5b:20:82";
  ip = null;
in
{
  den.aspects.infra._.clients._.server-vlan-download = {
    aspect-name = "infra.clients.server-vlan-download";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os = _: {
      infra.clients.server-vlan-download = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.server_vlan_download = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.server_vlan_download";
          id = mac;
        }
      ];
    };
  };
}
