# drinfernoo's "Dragon Ball" mixed list — Radarr pulls only the movies. Freeleech-gated.
_: {
  name = "Trakt Dragon Ball";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 11;
  limit = 500;
  username = "drinfernoo";
  listname = "dragonball";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.dragonball}" ];
}
