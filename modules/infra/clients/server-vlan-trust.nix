# home network
{ den, ... }:
let
  name = "Server Trust";
  mac = "62:f5:a4:5b:20:14";
  ip = null;
in
{
  den.aspects.infra._.clients._.server-vlan-trust = {
    aspect-name = "infra.clients.server-vlan-trust";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os = _: {
      infra.clients.server-vlan-trust = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.server_vlan_trust = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.server_vlan_trust";
          id = mac;
        }
      ];
    };
  };
}
