/*
  ha-seeders — declarative seeding for Home Assistant's .storage registries.

  HA has no declarative config for areas, config entries or persons, so these
  merge into /var/lib/hass/.storage/* before home-assistant.service starts.
  Each reads its seed data as JSON on stdin and is non-destructive: existing
  rows are merged or left alone, never replaced wholesale.

  Three binaries rather than one because each registry has its own merge
  semantics — areas append-only, persons never create (SSO owns them), config
  entries sync identity fields and replace subentries.
*/
{
  writers,
  symlinkJoin,
}:
let
  seeder = name: src: writers.writePython3Bin name { } (builtins.readFile src);
in
symlinkJoin {
  name = "ha-seeders";
  paths = [
    (seeder "ha-seed-areas" ./seed_areas.py)
    (seeder "ha-seed-config-entries" ./seed_config_entries.py)
    (seeder "ha-seed-persons" ./seed_persons.py)
  ];
  meta.description = "Home Assistant .storage registry seeders";
}
