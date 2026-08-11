/*
  kiwix-sync — keep Kiwix ZIM books in sync with an rsync mirror.
  https://www.kiwix.org

  The book list is argv (repeated --book category:prefix) rather than generated
  shell, so the host varies what is tracked without changing this script.
*/
{
  writeShellApplication,
  rsync,
  gawk,
  gnugrep,
  findutils,
  coreutils,
}:
writeShellApplication {
  name = "kiwix-sync";
  runtimeInputs = [
    rsync
    gawk
    gnugrep
    findutils
    coreutils
  ];
  text = builtins.readFile ./kiwix-sync.sh;
  meta.description = "Sync Kiwix ZIM books from an rsync mirror";
}
