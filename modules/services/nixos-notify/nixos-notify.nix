/*
  nixos-notify — reports host lifecycle events (boot, switch, scrub, gc) to the
  #nixos Matrix room by publishing {event, host, ...} to MQTT (homelab/notify/nixos);
  HA consumes via an mqtt trigger and owns formatting/routing. No sidecar on the host
  (unlike the containers), so it publishes to mosquitto directly via mosquitto_pub.
  Publishing to the broker (which stays up across an HA restart) is what fixes the
  old webhook race where a POST landed on HA mid-restart and was silently dropped.
*/
{ den, ... }:
{
  den.aspects.services._.nixos-notify = {
    aspect-name = "services.nixos-notify";
    includes = [ den.aspects.infra._.clients._.container-mosquitto ];

    nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        mq = config.infra.clients.container-mosquitto;
        mosquittoPub = "${pkgs.mosquitto}/bin/mosquitto_pub";
        topic = "homelab/notify/nixos";
        rev =
          if config.system.configurationRevision != null then
            config.system.configurationRevision
          else
            "dirty";
        report = "${pkgs.python3}/bin/python3 ${./nixos-notify.py} ${mosquittoPub} ${mq.ip} ${toString mq.ports.default} ${topic}";
        base = {
          after = [ "network-online.target" ];
          wants = [ "network-online.target" ];
        };
      in
      {
        systemd = {
          tmpfiles.rules = [ "d /var/lib/nixos-notify 0755 root root -" ];

          services = {
            nixos-notify-boot = base // {
              description = "Report host boot to #nixos via Home Assistant";
              wantedBy = [ "multi-user.target" ];
              path = [ pkgs.systemd ];
              serviceConfig = {
                Type = "oneshot";
                RemainAfterExit = true;
                ExecStart = "${report} boot";
              };
            };

            # Maintenance reports: hang off the existing scrub / gc units via OnSuccess
            # so they only post when the job actually completed cleanly.
            nixos-notify-scrub = base // {
              description = "Report a completed Btrfs scrub to #nixos via Home Assistant";
              path = [ pkgs.btrfs-progs ];
              serviceConfig = {
                Type = "oneshot";
                ExecStart = "${report} scrub /";
              };
            };

            nixos-notify-gc = base // {
              description = "Report a completed nix-gc to #nixos via Home Assistant";
              path = [ pkgs.systemd ];
              serviceConfig = {
                Type = "oneshot";
                ExecStart = "${report} gc";
              };
            };

            "btrfs-scrub--".onSuccess = lib.mkIf config.services.btrfs.autoScrub.enable [
              "nixos-notify-scrub.service"
            ];
            nix-gc.onSuccess = lib.mkIf config.nix.gc.automatic [ "nixos-notify-gc.service" ];
          };
        };

        # Fire the switch report on every activation (boot + nixos-rebuild switch)
        # as a detached transient unit. systemd-run is immune to switch-to-
        # configuration's job management (a plain restart gets cancelled) and
        # returns immediately so the network retry never blocks activation. The
        # script's state file dedupes, so it only posts on an actual closure change.
        #
        # Skipped under systemd stage-1: activation runs inside the initrd, where the
        # transient unit's default deps pull sysinit.target back up after initrd-cleanup
        # tore it down. systemd-sysctl then re-runs against the initrd's empty
        # /etc/sysctl.d and crosses switch-root already "finished", so stage 2 never
        # applies any sysctl — including kernel.modprobe, which kills module autoload.
        # The boot report is covered by nixos-notify-boot.service in stage 2 anyway.
        system.activationScripts.nixosNotifySwitch = ''
          if [ -z "''${IN_NIXOS_SYSTEMD_STAGE1:-}" ]; then
            ${pkgs.systemd}/bin/systemd-run --collect --quiet --unit=nixos-notify-switch \
              --setenv=PATH=${
                lib.makeBinPath [
                  pkgs.nvd
                  pkgs.nix
                  pkgs.coreutils
                ]
              } \
              ${report} switch ${lib.escapeShellArg rev} "$systemConfig" || true
          fi
        '';
      };
  };
}
