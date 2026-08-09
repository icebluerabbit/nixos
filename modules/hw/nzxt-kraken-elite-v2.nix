/*
  nzxt-kraken-elite-v2 — NZXT Kraken Elite liquid cooler profile, configuring fan curves and LCD display via CoolerControl
  https://gitlab.com/coolercontrol/coolercontrol
*/
{ den, ... }:
let
  krakenUid = "56560c774d8a21690967043f6aa1fb726ecfaddbf7e53a54c43e67ebfc0ec54b";
  cpuUid = "3145e1a42801faf9cf948df8c705afb4859f084c10a7e7c5e4efecb1e0167127";
  aioFanProfileUid = "4b3e2ecf-8883-4087-af80-48a95a6dd4fd";
  aioFunctionUid = "9f3148f6-b360-406e-9917-b015f3083197";
in
{
  den.aspects.hw._.nzxt-kraken-elite-v2 = {
    aspect-name = "hw.nzxt-kraken-elite-v2";
    # The Kraken Elite is an LCD unit with no OpenRGB-addressable lighting;
    # OpenRGB 1.0rc3 doesn't enumerate it at all. CoolerControl owns it
    # exclusively (KrakenZ3 driver) — LCD, pump, and fan below.
    includes = [
      den.aspects.hw._.coolercontrol
    ];
    nixos = _: {
      hardware.coolercontrol = {
        functions = [
          {
            uid = aioFunctionUid;
            name = "AIO Function";
            f_type = "ExponentialMovingAvg";
            duty_minimum = 5;
            duty_maximum = 100;
            step_size_min_decreasing = 0;
            step_size_max_decreasing = 0;
            sample_window = 5;
            threshold_hopping = true;
          }
        ];
        profiles = [
          {
            uid = aioFanProfileUid;
            name = "AIO Fan";
            p_type = "Graph";
            speed_profile = [
              [
                40.0
                50
              ]
              [
                50.0
                60
              ]
              [
                60.0
                70
              ]
              [
                65.0
                80
              ]
              [
                70.0
                90
              ]
              [
                75.0
                100
              ]
            ];
            temp_source = {
              temp_name = "temp1";
              device_uid = cpuUid;
            };
            temp_min = 0.0;
            temp_max = 100.0;
            function_uid = aioFunctionUid;
            offset_profile = [ ];
          }
        ];
        deviceSettings.${krakenUid} = {
          pump.speed_fixed = 100;
          fan.profile_uid = aioFanProfileUid;
          lcd.lcd = {
            mode = "liquid";
            brightness = 100;
            orientation = 0;
            colors = [ ];
          };
        };
      };
    };
  };
}
