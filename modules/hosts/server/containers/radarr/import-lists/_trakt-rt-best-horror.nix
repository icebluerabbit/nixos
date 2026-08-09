_: {
  name = "Trakt RT Best Horror";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 7;
  limit = 500;
  username = "lish408";
  listname = "rotten-tomatoes-best-horror-movies-of-all-time";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.rt-horror}" ];
}
