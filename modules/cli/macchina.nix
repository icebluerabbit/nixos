/*
  macchina — fast system info fetcher written in Rust; theme and display config via TOML
  https://github.com/macchina-cli/macchina
*/
_: {
  den.aspects.cli._.macchina = {
    aspect-name = "cli.macchina";
    homeManager =
      {
        pkgs,
        lib,
        config,
        ...
      }:
      {
        programs.macchina = {
          enable = true;
          settings = {
            theme = "Default";
            show = [
              "Kernel"
              "Distribution"
              "WindowManager"
              "Terminal"
              "Shell"
              "DiskSpace"
              "Resolution"
              "Processor"
              "Memory"
              "GPU"
            ];
          };

          themes = {
            Default = {
              spacing = 2;
              key_color = "#00EAFF";
              separator = "";
              hide_ascii = true;

              box = {
                border = "rounded";
                visible = true;
                inner_margin = {
                  x = 0;
                  y = 0;
                };
              };

              palette = {
                type = "Full";
                glyph = "██▓";
                visible = true;
              };

              keys = {
                host = "";
                kernel = "󰌽";
                de = "";
                wm = "";
                distro = "";
                packages = "";
                shell = "";
                terminal = "";
                uptime = "";
                cpu = "";
                gpu = "";
                battery = " ";
                backlight = "󰃠 ";
                resolution = "󰍹";
                memory = "";
              };
            };
          };
        };

        programs.zsh.initContent = lib.mkIf config.programs.zsh.enable ''
          ${lib.getExe pkgs.macchina}
        '';
      };
  };
}
