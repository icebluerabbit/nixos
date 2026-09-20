# Repos watched by the github config entry and the GitHub relay automations.
# Public and non-archived only, so the PAT needs no scopes at all.
#
# Adding a repo here is picked up on the next switch (seed_config_entries.py
# syncs subentries). Removing one prunes the subentry, but its entities linger
# as stale `unavailable` rows until deleted in the UI; entity unique_ids are
# derived from the GitHub numeric repo id, so re-adding a repo reuses them.
{ lib }:
rec {
  repos = [
    "icebluerabbit/nixos"
    "icebluerabbit/configuratarr"
    "icebluerabbit/denful-networks"
    "icebluerabbit/openchamber-flake"
    "icebluerabbit/youtarr-flake"
    "icebluerabbit/lazylibrarian-flake"
  ];

  # Mirrors HA's entity_id slugification of the device name (the repo full_name),
  # e.g. "icebluerabbit/nixos" → "icebluerabbit_nixos". Entity ids are derived rather than
  # listed because state triggers can't wildcard; if HA ever slugifies these
  # differently the automations go quiet, so check one after the first switch.
  slug = s: lib.toLower (builtins.replaceStrings [ "/" "-" "." ] [ "_" "_" "_" ] s);

  # entity_id → repo, for the per-repo prefix in relayed messages.
  entityMap =
    suffix:
    lib.listToAttrs (
      map (r: {
        name = "sensor.${slug r}_${suffix}";
        value = r;
      }) repos
    );
}
