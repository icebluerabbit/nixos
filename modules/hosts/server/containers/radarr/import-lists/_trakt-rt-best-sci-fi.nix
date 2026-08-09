_: {
  name = "Trakt RT Best Sci-Fi";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 10;
  limit = 500;
  username = "nileshr";
  listname = "rotten-tomatoes-best-sci-fi-movies-of-all-time";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.rt-sci-fi}" ];
}
