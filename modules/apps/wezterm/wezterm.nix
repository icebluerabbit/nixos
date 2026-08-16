/*
  WezTerm — GPU-accelerated terminal emulator and multiplexer written in Rust
  https://github.com/wez/wezterm
  Supports a persistent mux server (wezterm-mux-server) for session persistence across launches.
*/
{ den, ... }:
{
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
          extraConfig = builtins.readFile ./config.lua;
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
