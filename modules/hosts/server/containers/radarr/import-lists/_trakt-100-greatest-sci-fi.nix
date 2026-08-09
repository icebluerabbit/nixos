_: {
  name = "Trakt 100 Greatest Sci-Fi";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 9;
  limit = 500;
  username = "codermike";
  listname = "100-greatest-sci-fi-movies";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.greatest-sci-fi}" ];
}
