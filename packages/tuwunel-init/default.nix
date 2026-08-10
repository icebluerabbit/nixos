/*
  tuwunel-init — declaratively provision tuwunel users, rooms and spaces.

  Runs after the homeserver is up and creates the declared users via
  tuwunel's Synapse-compatible shared-secret registration API, then the rooms
  and space hierarchy. Idempotent: an already-registered user or existing
  alias is skipped.

  Takes port, server name, admin user and the users/rooms JSON on argv; the
  registration secret and per-user passwords come from the environment.
*/
{ writers }:
writers.writePython3Bin "tuwunel-init" { } (builtins.readFile ./init.py)
