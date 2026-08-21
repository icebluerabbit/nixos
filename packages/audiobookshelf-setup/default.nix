/*
  audiobookshelf-setup — initialize a fresh Audiobookshelf server and export an
  API key for downstream consumers (configuratarr).
  https://www.audiobookshelf.org

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
  name = "audiobookshelf-setup";
  runtimeInputs = [
    curl
    jq
    coreutils
  ];
  text = builtins.readFile ./audiobookshelf-setup.sh;
  meta.description = "Initialize Audiobookshelf and export its API key";
}
