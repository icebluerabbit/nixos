# Trakt trending movies — freeleech-only, same gate as the IMDb list.
_: {
  name = "Trakt Trending Movies";
  implementation = "TraktPopularImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 4;
  limit = 500;
  trakt_list_type = 0; # Trending Movies
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.trakt-trending}" ];
}
