# home network
{ den, ... }:
let
  name = "Server DMZ";
  mac = "62:f5:a4:5b:20:64";
  ip = null;
in
{
  den.aspects.infra._.clients._.server-vlan-dmz = {
    aspect-name = "infra.clients.server-vlan-dmz";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os = _: {
      infra.clients.server-vlan-dmz = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.server_vlan_dmz = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.server_vlan_dmz";
          id = mac;
        }
      ];
    };
  };
}
