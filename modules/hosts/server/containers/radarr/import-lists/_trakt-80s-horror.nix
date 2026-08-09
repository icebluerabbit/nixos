_: {
  name = "Trakt 80s Horror";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 5;
  limit = 500;
  username = "canadutchy";
  listname = "80s-horror";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.80s-horror}" ];
}
