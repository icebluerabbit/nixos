/*
  whippit — wrapper around `whipper cd rip` (see modules/cli/whipper.nix for the
  drive config): rips into a throwaway /tmp/rip.XXXX staging dir (whipper builds
  the Artist/Album subdirs inside from MusicBrainz metadata), summarizes the
  AccurateRip results, saves that summary as whippit-summary.log in the album dir,
  then asks y/n to keep (merge-copy the album subdirs into /mnt/media/rips/ for the
  server's beets to import) or discard (rm -rf the staging dir).
  https://github.com/whipper-team/whipper
*/
{
  writeShellApplication,
  whipper,
  coreutils,
  findutils,
  gnugrep,
}:
writeShellApplication {
  name = "whippit";
  runtimeInputs = [
    whipper
    coreutils
    findutils
    gnugrep
  ];
  text = builtins.readFile ./whippit.sh;
  meta.description = "Rip a CD with whipper into a staging dir, summarize AccurateRip, keep or discard";
}
