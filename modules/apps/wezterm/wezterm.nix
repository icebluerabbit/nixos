/*
  WezTerm — GPU-accelerated terminal emulator and multiplexer written in Rust
  https://github.com/wez/wezterm
  Supports a persistent mux server (wezterm-mux-server) for session persistence across launches.
*/
{ den, inputs, ... }:
{
  flake-file.inputs = {
    tabline-wez = {
      url = "github:michaelbrusegard/tabline.wez";
      flake = false;
    };
  };

  den.aspects.apps._.wezterm = {
    aspect-name = "apps.wezterm";
    includes = [
      den.aspects.fonts._.jetbrains-mono
      den.aspects.cli._.clipboard
    ];

    homeManager =
      { pkgs, ... }:
      {
        home.sessionVariables = {
          TERMINAL = "wezterm";
          WGPU_BACKEND = "vulkan";
        };

        programs.wezterm = {
          enable = true;
          extraConfig = ''
            local wezterm = require('wezterm')
            wezterm.plugin = wezterm.plugin or {}
            wezterm.plugin.list = function()
              return {
                {
                  plugin_dir = "${inputs.tabline-wez}/dummy",
                  component = "dummy"
                }
              }
            end
            package.path = package.path .. ';${inputs.tabline-wez}/lua/?.lua;${inputs.tabline-wez}/plugin/?.lua'
            local tabline = dofile('${inputs.tabline-wez}/plugin/init.lua')

            ${builtins.readFile ./config.lua}
          '';
        };

        systemd.user.services.wezterm-mux-server = {
          Unit = {
            Description = "WezTerm Mux Server";
            After = [ "graphical-session.target" ];
          };
          Install.WantedBy = [ "graphical-session.target" ];
          Service = {
            ExecStart = "${pkgs.wezterm}/bin/wezterm-mux-server --daemonize --foreground";
            Restart = "on-failure";
          };
        };
      };
  };
}
