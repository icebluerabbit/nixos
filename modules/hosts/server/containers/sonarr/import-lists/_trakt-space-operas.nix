# wanderlustlover's "Sci-Fi Space Operas" mixed list — Sonarr pulls only the shows.
# Freeleech-gated. Public user list → TraktListImport (username/listname).
_: {
  name = "Trakt Space Operas";
  implementation = "TraktListImport";
  enable_automatic_add = true;
  search_for_missing_episodes = false;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  series_type = "standard";
  season_folder = true;
  list_order = 7;
  limit = 500;
  username = "wanderlustlover-gmail-com";
  listname = "scifi-space-operas";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.space-operas}" ];
}
