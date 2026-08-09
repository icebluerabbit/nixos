/*
  sensor — lm_sensors for hardware temperature and voltage monitoring
  https://github.com/lm-sensors/lm-sensors
*/
_: {
  den.aspects.hw._.sensor = {
    aspect-name = "hw.sensor";
    nixos =
      { pkgs, ... }:
      {
        hardware.sensor.hddtemp.enable = false;
        environment.systemPackages = [ pkgs.lm_sensors ];
      };
  };
}
