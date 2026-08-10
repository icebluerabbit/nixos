/*
  thelounge-seed — enforce declared IRC networks into thelounge's user files.

  thelounge stores per-user networks in users/<name>.json, which it rewrites
  at runtime, so the declared set is merged back in on every boot rather than
  templated once.

  Seed data arrives as JSON via $THELOUNGE_SEED_FILE; takes no arguments.
*/
{ writers }:
writers.writePython3Bin "thelounge-seed" { } (builtins.readFile ./seed.py)
