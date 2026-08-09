# drinfernoo's "Dragon Ball" mixed list — Sonarr pulls only the shows. Freeleech-gated.
_: {
  name = "Trakt Dragon Ball";
  implementation = "TraktListImport";
  enable_automatic_add = true;
  search_for_missing_episodes = false;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  series_type = "anime";
  season_folder = true;
  list_order = 5;
  limit = 500;
  username = "drinfernoo";
  listname = "dragonball";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.dragonball}" ];
}
