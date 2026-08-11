/*
  lazylibrarian-notify — LazyLibrarian notify hook that emits the book's DB row
  as JSON to an HTTP endpoint (this homelab points it at the Vector sidecar,
  which republishes to MQTT).
  https://gitlab.com/LazyLibrarian/LazyLibrarian

  LazyLibrarian owns argv, so the db path and endpoint arrive before a `--`
  separator; see the wrapper in the lazylibrarian host aspect.
*/
{
  writeShellApplication,
  sqlite,
  jq,
  curl,
  coreutils,
}:
writeShellApplication {
  name = "lazylibrarian-notify";
  runtimeInputs = [
    sqlite
    jq
    curl
    coreutils
  ];
  text = builtins.readFile ./lazylibrarian-notify.sh;
  meta.description = "LazyLibrarian notify hook: DB row -> JSON -> HTTP POST";
}
