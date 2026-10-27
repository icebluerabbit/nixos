/*
  fscache-textfile — scrape /proc/fs/fscache/stats into a node-exporter
  textfile-collector .prom file.

  The kernel exposes fscache counters only through procfs; node-exporter has no
  fscache collector. Paths are argv, so nothing here knows about a particular
  host.
*/
{
  writeShellApplication,
  gawk,
  coreutils,
}:
writeShellApplication {
  name = "fscache-textfile";
  runtimeInputs = [
    gawk
    coreutils
  ];
  text = builtins.readFile ./fscache-textfile.sh;
  meta.description = "Export fscache stats as node-exporter textfile metrics";
}
