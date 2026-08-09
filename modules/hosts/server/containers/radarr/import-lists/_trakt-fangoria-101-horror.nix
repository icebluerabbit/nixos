_: {
  name = "Trakt Fangoria 101 Horror";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 6;
  limit = 500;
  username = "sp1ti";
  listname = "fangoria-s-101-best-horror-movies-you-ve-never-seen";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.fangoria-horror}" ];
}
