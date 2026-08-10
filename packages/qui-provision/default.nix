/*
  qui-provision — register the qBittorrent instance in qui at startup.

  qui keeps instances in its SQLite DB with no declarative config, so the
  instance is created over the REST API instead. Idempotent: bails if an
  instance already targets the same host.

  Usage: qui-provision <api-base> <qbt-url> <name>
*/
{ writers }:
writers.writePython3Bin "qui-provision" { } (builtins.readFile ./qui_provision.py)
