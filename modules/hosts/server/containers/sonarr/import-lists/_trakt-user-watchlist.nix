# Personal watchlist — grab anything (Any profile) and search for missing. NOT
# freeleech-gated; titles I explicitly picked.
_: {
  name = "Trakt User";
  implementation = "TraktUserImport";
  enable_automatic_add = true;
  search_for_missing_episodes = true;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = 1; # Any
  series_type = "standard";
  season_folder = true;
  list_order = 1;
  limit = 1000;
  auth_user = "\${env.TRAKT_AUTH_USER}";
  trakt_list_type = 0; # User Watch List
  tags = [ "\${ref.tag.watchlist}" ];
}
