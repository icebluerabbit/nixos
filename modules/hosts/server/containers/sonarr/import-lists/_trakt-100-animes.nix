# rfsbraz's "100 Animes to Watch Before You Die" — freeleech-gated, same as the
# other curated Trakt lists. Public user list → TraktListImport (username/listname).
_: {
  name = "Trakt 100 Animes";
  implementation = "TraktListImport";
  enable_automatic_add = true;
  search_for_missing_episodes = false;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  series_type = "anime";
  season_folder = true;
  list_order = 4;
  limit = 500;
  username = "rfsbraz";
  listname = "100-animes-to-watch-before-you-die";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.anime}" ];
}
