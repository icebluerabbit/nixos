# wanderlustlover's "Sci-Fi Space Operas" mixed list — Radarr pulls only the movies.
# Freeleech-gated. Public user list → TraktListImport (username/listname).
_: {
  name = "Trakt Space Operas";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 13;
  limit = 500;
  username = "wanderlustlover-gmail-com";
  listname = "scifi-space-operas";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.space-operas}" ];
}
