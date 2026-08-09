# home network
{ den, ... }:
let
  name = "Presence Sensor FP2 96E8";
  mac = "54:ef:44:a1:96:e8";
  ip = "10.10.30.117";
in
{
  den.aspects.infra._.clients._.presence-sensor-fp2-96e8 = {
    aspect-name = "infra.clients.presence-sensor-fp2-96e8";
    includes = [
      den.aspects.infra._.providers._.unifi
      den.aspects.infra._.options
    ];

    os = _: {
      infra.clients.presence-sensor-fp2-96e8 = {
        inherit name mac ip;
      };
    };

    terranix = {
      resource.unifi_client.presence_sensor_fp2_96e8 = {
        inherit mac name;
        fixed_ip = ip;
        allow_existing = true;
        blocked = false;
        site = "default";
        skip_forget_on_destroy = false;
      };

      import = [
        {
          to = "unifi_client.presence_sensor_fp2_96e8";
          id = mac;
        }
      ];
    };
  };
}
