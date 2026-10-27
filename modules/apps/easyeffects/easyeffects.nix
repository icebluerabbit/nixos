/*
  EasyEffects — non-destructive audio effects processor for PipeWire
  https://github.com/wwmm/easyeffects
  GTK4 app; operates as a PipeWire filter chain via a virtual sink node.
*/
_: {
  den.aspects.apps._.easyeffects = {
    aspect-name = "apps.easyeffects";
    nixos = _: {
      programs.dconf.enable = true;

      services.pipewire.extraConfig.pipewire."99-easyeffects" = {
        "rules" = [
          {
            matches = [ { "node.name" = "easyeffects_sink"; } ];
            actions = {
              update-props = {
                "priority.session" = 2000;
              };
            };
          }
        ];
      };
    };

    homeManager =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        # `--load-preset` at service start races the PipeWire graph: EE issues the
        # load before its own easyeffects_sink filter has registered, so the
        # effects chain (equalizer node) never instantiates and audio passes
        # through unprocessed. This bit after a mid-session crash-restart, when
        # PipeWire was already up — so daemon ordering alone can't fix it.
        # `easyeffects -l` always exits 0, so easyeffects-reload polls the graph
        # for the equalizer node and re-issues the load until it appears.
        #
        # Deliberately NOT an ExecStartPost of easyeffects.service. EE 8.x does
        # single-instance IPC over a QLocalServer socket, and `easyeffects -l`
        # with no live server becomes a second full instance instead of failing
        # — so as an ExecStartPost it blocked inside the 90s start timeout and
        # took the daemon down with it (`start-post operation timed out`, ~300ms
        # CPU over 90s wall). The .path unit below waits for that socket instead.
        reloadPreset = lib.escapeShellArgs [
          (lib.getExe pkgs.homelab.easyeffects-reload)
          "--easyeffects"
          "${config.services.easyeffects.package}/bin/easyeffects"
          "--preset"
          "basssss"
          "--node"
          "ee_soe_equalizer"
        ];
      in
      {
        services.easyeffects = {
          enable = true;
          preset = "basssss";
          extraPresets.basssss = builtins.fromJSON (builtins.readFile ./bass-preset.json);
        };

        systemd.user = {
          services.easyeffects = {
            Unit.After = [
              "pipewire.service"
              "wireplumber.service"
              "pipewire-pulse.service"
            ];
            Unit.Wants = [
              "pipewire.service"
              "wireplumber.service"
            ];
          };

          # inotify on the single-instance socket, so the reload fires the moment
          # EE is actually able to answer a client call. %t is XDG_RUNTIME_DIR.
          paths.easyeffects-reload = {
            Unit = {
              Description = "Watch for the EasyEffects IPC socket";
              PartOf = [ "easyeffects.service" ];
            };
            Path.PathExists = "%t/EasyEffectsServer";
            Install.WantedBy = [ "graphical-session.target" ];
          };

          services.easyeffects-reload = {
            Unit = {
              Description = "Re-issue the EasyEffects preset load until the graph has it";
              # PartOf so a daemon restart re-runs this; the socket usually still
              # exists at that moment, which is why the script re-checks it.
              PartOf = [ "easyeffects.service" ];
              After = [ "easyeffects.service" ];
            };
            Service = {
              Type = "oneshot";
              # PathExists is level-triggered: without RemainAfterExit the .path
              # unit would retrigger the instant this exits, in a hot loop.
              RemainAfterExit = true;
              # Budget: 10s socket wait + 12 * (3s call + 0.5s) = 52s.
              TimeoutStartSec = 60;
              ExecStart = reloadPreset;
            };
          };
        };
      };
  };
}
