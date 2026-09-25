/*
  nzxt-rgb-fan-controller — NZXT RGB & Fan Controller profile, configuring fan curves via CoolerControl and lighting via OpenRGB
  https://gitlab.com/coolercontrol/coolercontrol
*/
{ den, ... }:
let
  uid = "7ceb77ac2104d6434b22f701dd5f96ebc5cc872553f371059b0f5173c0957f97";
  cpuUid = "3145e1a42801faf9cf948df8c705afb4859f084c10a7e7c5e4efecb1e0167127";
  gpuUid = "7e51251c32d94c13a4a7fcfc30a4c8afd505f84019f61b924284334f62db9c21";
  fan1ProfileUid = "1d008ed5-d09b-4368-987a-74bb7701c58b";
  fan2ProfileUid = "94c1636d-c6ef-4e5c-bc78-a8f982bc0912";
  fan3ProfileUid = "e61739ec-8564-4c3a-8b72-a8eddd323bd5";
  fanCtrlFunctionUid = "eb6c50ca-43cf-489b-9ea3-fe9959cf927b";
in
{
  den.aspects.hw._.nzxt-rgb-fan-controller = {
    aspect-name = "hw.nzxt-rgb-fan-controller";
    includes = [
      den.aspects.hw._.rgb
      den.aspects.hw._.coolercontrol
    ];
    nixos =
      { pkgs, lib, ... }:
      {
        systemd.services.nzxt-fan-controller-rgb = {
          wantedBy = [ "multi-user.target" ];
          after = [
            "openrgb.service"
            "coolercontrold.service"
          ];
          requires = [ "openrgb.service" ];
          serviceConfig = {
            Type = "oneshot";
            RemainAfterExit = true;
            # NZXT Hue2 can enumerate in the OpenRGB SDK server well past 60s on
            # a cold boot, so --retries covers the whole 180s window.
            TimeoutStartSec = "180s";
            # One whole-device set, detected in-process (--local). The Hue 2
            # reports its own layout — 24 + 24 + 18 LEDs on channels 2/3/4 —
            # and a fresh detection reads that correctly, so nothing here has
            # to name zones. Writing through openrgb.service's long-lived SDK
            # view instead left whole channels dark, and the per-zone writes it
            # took to work around that are what made the strips flash on every
            # deploy: four device-wide resets in a row rather than one write.
            ExecStart = lib.escapeShellArgs [
              (lib.getExe pkgs.homelab.openrgb-init)
              "--openrgb"
              (lib.getExe pkgs.openrgb)
              "--local"
              "--device"
              "NZXT RGB & Fan Controller"
              "--retries"
              "180"
              "--settle"
              "3"
              "--passes"
              "2"
              "--set"
              ":ffffff"
              "--pass-delay"
              "1"
            ];
          };
        };

        hardware.coolercontrol = {
          functions = [
            {
              uid = fanCtrlFunctionUid;
              name = "Fan Ctrl Function";
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
              uid = fan1ProfileUid;
              name = "Back Fan (CPU)";
              p_type = "Graph";
              speed_profile = [
                [
                  35.0
                  15
                ]
                [
                  50.0
                  25
                ]
                [
                  65.0
                  50
                ]
                [
                  75.0
                  65
                ]
                [
                  85.0
                  85
                ]
                [
                  95.0
                  100
                ]
              ];
              temp_source = {
                temp_name = "temp1";
                device_uid = cpuUid;
              };
              temp_min = 0.0;
              temp_max = 100.0;
              function_uid = fanCtrlFunctionUid;
              offset_profile = [ ];
            }
            {
              uid = fan2ProfileUid;
              name = "Bottom Fans (GPU)";
              p_type = "Graph";
              speed_profile = [
                [
                  35.0
                  20
                ]
                [
                  55.0
                  35
                ]
                [
                  70.0
                  60
                ]
                [
                  80.0
                  80
                ]
                [
                  87.0
                  100
                ]
              ];
              temp_source = {
                temp_name = "GPU Temp";
                device_uid = gpuUid;
              };
              temp_min = 0.0;
              temp_max = 100.0;
              function_uid = fanCtrlFunctionUid;
              offset_profile = [ ];
            }
            {
              uid = fan3ProfileUid;
              name = "Side Fans (CPU)";
              p_type = "Graph";
              speed_profile = [
                [
                  40.0
                  40
                ]
                [
                  45.0
                  50
                ]
                [
                  50.0
                  60
                ]
                [
                  55.0
                  70
                ]
                [
                  60.0
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
              function_uid = fanCtrlFunctionUid;
              offset_profile = [ ];
            }
          ];
          deviceSettings.${uid} = {
            fan1.profile_uid = fan1ProfileUid;
            fan2.profile_uid = fan2ProfileUid;
            fan3.profile_uid = fan3ProfileUid;
          };
        };
      };
  };
}
