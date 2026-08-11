/*
  restic-backup — the shared body of this homelab's restic jobs.
  https://restic.net

    restic-backup              init-if-needed, back up, forget+prune, publish
                               a JSON success payload to MQTT
    restic-backup-notify-fail  publish a failure payload with the failed unit's
                               journal tail (runs from OnFailure=)

  Tag, excludes, retention, broker and paths are all argv, and the repository
  comes from the standard $RESTIC_* environment the unit already sets — so the
  desktop's /projects job and the server's appdata job are the same binary with
  different flags.
*/
{
  writeShellApplication,
  symlinkJoin,
  restic,
  jq,
  mosquitto,
  systemd,
  coreutils,
}:
let
  common = [
    restic
    jq
    mosquitto
    coreutils
  ];
in
symlinkJoin {
  name = "restic-backup";
  paths = [
    (writeShellApplication {
      name = "restic-backup";
      runtimeInputs = common;
      text = builtins.readFile ./backup.sh;
    })
    (writeShellApplication {
      name = "restic-backup-notify-fail";
      # journalctl, for the failed unit's log tail
      runtimeInputs = common ++ [ systemd ];
      text = builtins.readFile ./notify-fail.sh;
    })
  ];
  meta.description = "restic backup + MQTT result reporting";
}
