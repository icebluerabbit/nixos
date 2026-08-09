# istoit's "Sci-Fi TV Shows" — freeleech-gated, same as the other curated Trakt
# lists. Public user list → TraktListImport (username/listname).
_: {
  name = "Trakt Sci-Fi TV";
  implementation = "TraktListImport";
  enable_automatic_add = true;
  search_for_missing_episodes = false;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  series_type = "standard";
  season_folder = true;
  list_order = 6;
  limit = 500;
  username = "istoit";
  listname = "scifi-tv-shows";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.scifi-tv}" ];
}
