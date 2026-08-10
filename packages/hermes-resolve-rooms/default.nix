/*
  hermes-resolve-rooms — resolve Matrix room aliases to room IDs for Hermes.

  Matrix mints room IDs server-side at createRoom, so only the alias is
  knowable at eval time. This resolves the declared aliases and appends the
  resulting IDs to the .env the hermes-agent activation script just wrote.
  Unresolved aliases are not fatal.

  Usage: hermes-resolve-rooms <homeserver> <env_file> <home_alias|-> [alias…]
*/
{ writers }:
writers.writePython3Bin "hermes-resolve-rooms" { } (builtins.readFile ./resolve_rooms.py)
