/*
  komga-setup — claim a fresh Komga server and export an API key for downstream
  consumers (configuratarr).
  https://komga.org

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
  name = "komga-setup";
  runtimeInputs = [
    curl
    jq
    coreutils
  ];
  text = builtins.readFile ./komga-setup.sh;
  meta.description = "Claim a Komga server and export its API key";
}
