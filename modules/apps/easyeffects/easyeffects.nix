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
      { config, pkgs, ... }:
      let
        # `--load-preset` at service start races the PipeWire graph: EE issues the
        # load before its own easyeffects_sink filter has registered, so the
        # effects chain (equalizer node) never instantiates and audio passes
        # through unprocessed. This bit after a mid-session crash-restart, when
        # PipeWire was already up — so daemon ordering alone can't fix it.
        # `easyeffects -l` always exits 0, so poll the graph for the equalizer
        # node and re-issue the load until it actually appears.
        reloadPreset = pkgs.writeShellScript "easyeffects-reload-basssss" ''
          for _ in $(seq 1 30); do
            ${config.services.easyeffects.package}/bin/easyeffects -l basssss || true
            if ${pkgs.pipewire}/bin/pw-cli list-objects Node 2>/dev/null \
              | grep -q ee_soe_equalizer; then
              exit 0
            fi
            sleep 0.5
          done
          exit 0
        '';
      in
      {
        services.easyeffects = {
          enable = true;
          preset = "basssss";
          extraPresets.basssss = builtins.fromJSON (builtins.readFile ./bass-preset.json);
        };

        systemd.user.services.easyeffects = {
          Unit.After = [
            "pipewire.service"
            "wireplumber.service"
            "pipewire-pulse.service"
          ];
          Unit.Wants = [
            "pipewire.service"
            "wireplumber.service"
          ];
          Service.ExecStartPost = [ "${reloadPreset}" ];
        };
      };
  };
}
