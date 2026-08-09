# Trakt trending shows — freeleech-only, same gate as the IMDb list.
_: {
  name = "Trakt Trending Shows";
  implementation = "TraktPopularImport";
  enable_automatic_add = true;
  search_for_missing_episodes = false;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  series_type = "standard";
  season_folder = true;
  list_order = 3;
  limit = 500;
  trakt_list_type = 0; # Trending Shows
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.trakt-trending}" ];
}
