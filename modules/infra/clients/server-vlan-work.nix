# home network
{ den, ... }:
let
  name = "Server Work";
  mac = "62:f5:a4:5b:20:46";
  ip = null;
in
{
  den.aspects.infra._.clients._.server-vlan-work = {
    aspect-name = "infra.clients.server-vlan-work";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os = _: {
      infra.clients.server-vlan-work = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.server_vlan_work = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.server_vlan_work";
          id = mac;
        }
      ];
    };
  };
}
