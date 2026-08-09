# volume-notifications — background service that listens for PipeWire volume changes and displays them via Mako
{ den, ... }:
{
  den.aspects.services._.volume-notifications = {
    aspect-name = "services.volume-notifications";
    includes = [
      den.aspects.gui._.mako
      den.aspects.hw._.audio
    ];

    homeManager =
      {
        pkgs,
        lib,
        ...
      }:
      {
        systemd.user.services.volume-notifications = {
          Unit = {
            Description = "Volume change notification listener";
            After = [
              "graphical-session.target"
              "mako.service"
            ];
            PartOf = [ "graphical-session.target" ];
          };
          Service = {
            ExecStart =
              let
                wpctl = lib.getExe' pkgs.wireplumber "wpctl";
                pactl = lib.getExe' pkgs.pulseaudio "pactl";
                notify-send = lib.getExe pkgs.libnotify;
                awk = lib.getExe pkgs.gawk;
                grep = lib.getExe pkgs.gnugrep;

                script = pkgs.writeShellScript "volume-notifications" ''
                  # Initialize state
                  last_state=$(${wpctl} get-volume @DEFAULT_AUDIO_SINK@)

                  # Listen for changes via pactl subscribe
                  # We filter for "sink" events which include volume/mute changes
                  ${pactl} subscribe | while read -r event; do
                    if echo "$event" | ${grep} -q "on sink"; then
                      current_state=$(${wpctl} get-volume @DEFAULT_AUDIO_SINK@)
                      
                      if [ "$current_state" != "$last_state" ]; then
                        # Parse volume (e.g., "Volume: 0.50")
                        volume=$(echo "$current_state" | ${awk} '{print int($2 * 100)}')
                        
                        # Check if muted
                        if echo "$current_state" | ${grep} -q "\[MUTED\]"; then
                          ${notify-send} -a "System" \
                            -h string:x-canonical-private-synchronous:volume \
                            -i audio-volume-muted \
                            -t 2000 \
                            "Volume: Muted"
                        else
                          # Determine icon based on volume level
                          icon="audio-volume-low"
                          if [ "$volume" -gt 66 ]; then
                            icon="audio-volume-high"
                          elif [ "$volume" -gt 33 ]; then
                            icon="audio-volume-medium"
                          fi

                          ${notify-send} -a "System" \
                            -h string:x-canonical-private-synchronous:volume \
                            -h int:value:"$volume" \
                            -i "$icon" \
                            -t 2000 \
                            "Volume: $volume%"
                        fi
                        last_state="$current_state"
                      fi
                    fi
                  done
                '';
              in
              "${script}";
            Restart = "always";
            RestartSec = 3;
          };
          Install.WantedBy = [ "graphical-session.target" ];
        };
      };
  };
}
