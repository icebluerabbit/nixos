/*
  mako — lightweight Wayland notification daemon; implements org.freedesktop.Notifications via D-Bus
  https://github.com/emersion/mako
  Supports criteria-based filtering, Pango markup, and per-urgency config.
*/
{ den, ... }:
{
  den.aspects.gui._.mako = {
    aspect-name = "gui.mako";
    includes = [
      den.aspects.gui._.wayland
    ];

    homeManager =
      { pkgs, lib, ... }:
      {
        home.packages = [
          pkgs.libnotify
          pkgs.papirus-icon-theme
        ];

        services.mako = {
          enable = true;
          settings = {
            # --- Position ---
            anchor = "top-center"; # top-right|top-center|top-left|bottom-right|bottom-center|bottom-left|center-right|center-left|center
            layer = "top"; # background|bottom|top|overlay (overlay = above fullscreen)
            # output = "";                  # output name, empty = focused output (default)

            # --- Layout ---
            width = 320; # default: 300
            height = 100; # max height; shrinks to fit
            outer-margin = 5; # margin around the whole notification list (30px bar + 10px gap)
            margin = 10; # margin around each individual notification
            padding = "10,14"; # inner padding; directional: top,right,bottom,left

            # --- Borders ---
            border-size = 2;
            border-radius = 12; # default: 0; directional supported

            # --- Colors (stylix manages: background-color, text-color, border-color, progress-color, font) ---

            # --- Icons ---
            icons = 1;
            max-icon-size = 64;
            icon-location = "left"; # left|right|top|bottom
            icon-border-radius = 0;
            icon-path = "${pkgs.papirus-icon-theme}/share/icons/Papirus-Dark";

            # --- Text ---
            markup = 1; # enable Pango markup in format and body
            format = "<b>%s</b>\\n%b"; # %a=app %s=summary %b=body %g=group-count %i=id
            text-alignment = "left"; # left|center|right

            # --- Timeout ---
            default-timeout = 5000; # ms; 0 = no timeout
            ignore-timeout = 0; # ignore app-provided timeout, use default-timeout

            # --- Grouping ---
            max-visible = 5; # max visible notifications; -1 = unlimited
            # group-by = "";               # comma-separated fields to group by: app-name|summary|body|urgency|category|desktop-entry

            # --- History ---
            history = 1;
            max-history = 5;

            # --- Sort ---
            sort = "-time"; # +/-time | +/-priority

            # --- Actions ---
            actions = 1;

            # --- Buttons ---
            on-button-left = "invoke-default-action";
            on-button-middle = "none";
            on-button-right = "dismiss";
            on-touch = "dismiss";
            on-notify = "none";

            # --- Criteria: urgency overrides (stylix sets colors, so only override non-color settings here) ---
            "urgency=low" = {
              default-timeout = 3000;
            };

            "urgency=normal" = {
              default-timeout = 5000;
            };

            "urgency=critical" = {
              default-timeout = 10000;
            };
          };
        };

        systemd.user.services.mako = {
          Unit = {
            Description = "Mako notification daemon";
            After = [ "graphical-session.target" ];
            PartOf = [ "graphical-session.target" ];
          };
          Service = {
            Type = "dbus";
            BusName = "org.freedesktop.Notifications";
            ExecStart = lib.getExe pkgs.mako;
            ExecReload = "${pkgs.mako}/bin/makoctl reload";
          };
          Install.WantedBy = [ "graphical-session.target" ];
        };
      };
  };
}
