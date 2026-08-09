/*
  picard — MusicBrainz GUI tagger with AcoustID fingerprinting. Manual
  match/lookup fallback for the rare CD rip whipper + beets can't auto-identify:
  fingerprint or hand-pick the release, save tags, then drop into /mnt/media/rips
  so beets sails through.
  https://picard.musicbrainz.org
*/
_: {
  den.aspects.apps._.picard = {
    aspect-name = "apps.picard";
    homeManager =
      { pkgs, lib, ... }:
      lib.mkIf pkgs.stdenv.hostPlatform.isLinux {
        home.packages = [ pkgs.picard ];
      };
  };
}
