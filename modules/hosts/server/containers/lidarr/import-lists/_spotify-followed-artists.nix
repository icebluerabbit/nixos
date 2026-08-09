# Spotify Followed Artists — pulls every artist you follow on Spotify into the library.
#
# The OAuth triple (accessToken/refreshToken/expires) is deliberately NOT declared here.
# Spotify lists authenticate by an interactive sign-in through Lidarr's UI, not by an
# API key, and Lidarr rotates the refresh token as it renews — so pinning tokens in Nix
# would push a stale token back over the fresh one on the next sync and break the list.
# configuratarr's merge is sparse (live base, desired wins, provider `fields[]` merged by
# name with live-only entries kept), so omitting them leaves the UI-minted tokens intact.
#
# CONSEQUENCE: this list must exist and be signed-in in the UI *before* this file syncs.
# The natural key is `name` — it must match the UI's name exactly, or configuratarr will
# create a second, token-less (dead) list instead of adopting the authenticated one.
#
# should_search = false: should_monitor = "entireArtist" monitors each artist's whole back
# catalogue, and searching on add would fire one search per missing album on first sync.
# Missing albums instead sit in Wanted and fill in via RSS/autobrr, or a manual search.
_: {
  name = "Spotify Followed Artists";
  implementation = "SpotifyFollowedArtists";
  enable_automatic_add = true;
  should_monitor = "entireArtist";
  monitor_new_items = "all";
  should_monitor_existing = false;
  should_search = false;
  root_folder_path = "/mnt/media/library/music";
  quality_profile_id = "\${ref.quality_profile.Any}";
  metadata_profile_id = "\${ref.metadata_profile.Standard}";
  list_order = 1;
}
