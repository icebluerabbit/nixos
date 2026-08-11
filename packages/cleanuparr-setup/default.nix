/*
  cleanuparr-setup — complete Cleanuparr's first-run setup and export its API key
  for downstream consumers (configuratarr).
  https://github.com/Cleanuparr/Cleanuparr

  Admin credentials stay in the environment; only the base URL and the key file
  path are argv.
*/
{
  writeShellApplication,
  curl,
  jq,
  coreutils,
}:
writeShellApplication {
  name = "cleanuparr-setup";
  runtimeInputs = [
    curl
    jq
    coreutils
  ];
  text = builtins.readFile ./cleanuparr-setup.sh;
  meta.description = "Complete Cleanuparr first-run setup and export its API key";
}
