# home network
{ den, ... }:
let
  name = "Server Services";
  mac = "62:f5:a4:5b:20:8c";
  ip = null;
in
{
  den.aspects.infra._.clients._.server-vlan-services = {
    aspect-name = "infra.clients.server-vlan-services";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options

    ];

    os = _: {
      infra.clients.server-vlan-services = {
        inherit
          name
          mac
          ip
          ;
      };
    };

    terranix = {
      resource.unifi_client.server_vlan_services = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.server_vlan_services";
          id = mac;
        }
      ];
    };
  };
}
