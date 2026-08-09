_: {
  name = "Trakt Hidden Horror Gems";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 8;
  limit = 500;
  username = "phantasmx1975";
  listname = "hidden-horror-gems";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.hidden-horror}" ];
}
