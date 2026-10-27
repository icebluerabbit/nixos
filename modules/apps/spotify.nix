/*
  Spotify — the official desktop client
  https://www.spotify.com
  Unfree: a repackaged upstream binary, so there is nothing to configure
  declaratively — settings live in the app's own account state.
*/
_: {
  den.aspects.apps._.spotify = {
    aspect-name = "apps.spotify";
    homeManager =
      { pkgs, ... }:
      {
        home.packages = [ pkgs.spotify ];
      };
  };
}
