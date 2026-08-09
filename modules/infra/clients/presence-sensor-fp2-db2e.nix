# home network
{ den, ... }:
let
  name = "Presence Sensor FP2 DB2E";
  mac = "54:ef:44:a3:db:2e";
  ip = "10.10.30.165";
in
{
  den.aspects.infra._.clients._.presence-sensor-fp2-db2e = {
    aspect-name = "infra.clients.presence-sensor-fp2-db2e";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.presence-sensor-fp2-db2e = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.presence_sensor_fp2_db2e = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.presence_sensor_fp2_db2e";
          id = mac;
        }
      ];
    };
  };
}
