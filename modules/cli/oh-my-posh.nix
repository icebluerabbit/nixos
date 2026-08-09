/*
  Oh My Posh — cross-shell prompt engine with JSON/TOML/YAML theme config; renders segments via Go templates
  https://github.com/JanDeDobbeleer/oh-my-posh
*/
_: {
  den.aspects.cli._.oh-my-posh = {
    aspect-name = "cli.oh-my-posh";
    homeManager =
      { config, ... }:
      {
        programs.oh-my-posh = {
          enable = true;
          enableZshIntegration = config.programs.zsh.enable;
          settings = {
            "$schema" = "https://raw.githubusercontent.com/JanDeDobbeleer/oh-my-posh/main/themes/schema.json";
            version = 4;
            final_space = true;
            streaming = 100;
            transient_prompt = {
              background = "transparent";
              foreground = "#${config.lib.stylix.colors.base0E}";
              template = "# ";
            };
            blocks = [
              {
                alignment = "left";
                type = "prompt";
                segments = [
                  {
                    type = "text";
                    style = "diamond";
                    leading_diamond = "";
                    trailing_diamond = " ";
                    background = "#${config.lib.stylix.colors.base08}";
                    foreground = "#${config.lib.stylix.colors.base00}";
                    template = "{{ if .Env.SSH_CLIENT }} 󰣀 {{ end }}";
                  }
                  {
                    type = "path";
                    style = "diamond";
                    leading_diamond = "";
                    trailing_diamond = "";
                    powerline_symbol = "";
                    background = "#${config.lib.stylix.colors.base0E}";
                    foreground = "#${config.lib.stylix.colors.base00}";
                    template = "  {{ .Path }} ";
                    options = {
                      style = "folder";
                    };
                  }
                  {
                    type = "git";
                    style = "powerline";
                    powerline_symbol = "";
                    background = "#${config.lib.stylix.colors.base0A}";
                    foreground = "#${config.lib.stylix.colors.base00}";
                    background_templates = [
                      "{{ if or (.Working.Changed) (.Staging.Changed) }}#${config.lib.stylix.colors.base09}{{ end }}"
                      "{{ if and (gt .Ahead 0) (gt .Behind 0) }}#${config.lib.stylix.colors.base08}{{ end }}"
                      "{{ if gt .Ahead 0 }}#${config.lib.stylix.colors.base0C}{{ end }}"
                      "{{ if gt .Behind 0 }}#${config.lib.stylix.colors.base09}{{ end }}"
                    ];
                    template = " {{ .UpstreamIcon }}{{ .HEAD }}{{if .BranchStatus }} {{ .BranchStatus }}{{ end }}{{ if .Staging.Changed }}  {{ .Staging.String }}{{ end }}{{ if and (.Working.Changed) (.Staging.Changed) }} |{{ end }}{{ if .Working.Changed }}  {{ .Working.String }}{{ end }}{{ if gt .StashCount 0 }}  {{ .StashCount }}{{ end }} ";
                    options = {
                      fetch_status = true;
                      fetch_upstream_icon = true;
                    };
                  }
                  {
                    type = "status";
                    style = "diamond";
                    leading_diamond = "<transparent,background></>";
                    trailing_diamond = "";
                    background = "#${config.lib.stylix.colors.base0D}";
                    foreground = "#${config.lib.stylix.colors.base05}";
                    background_templates = [
                      "{{ if gt .Code 0 }}#${config.lib.stylix.colors.base08}{{ end }}"
                    ];
                    template = "  ";
                    options = {
                      always_enabled = true;
                    };
                  }
                ];
              }
              {
                alignment = "left";
                type = "prompt";
                newline = true;
                segments = [
                  {
                    type = "root";
                    style = "plain";
                    foreground = "#${config.lib.stylix.colors.base08}";
                    template = " ";
                  }
                  {
                    type = "text";
                    style = "plain";
                    foreground = "#${config.lib.stylix.colors.base0E}";
                    template = "# ";
                  }
                ];
              }
            ];
          };
        };
      };
  };
}
