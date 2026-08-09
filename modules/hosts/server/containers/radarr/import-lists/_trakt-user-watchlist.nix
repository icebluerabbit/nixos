# Personal watchlist — grab anything (Any profile) and search on add. NOT
# freeleech-gated; these are titles I explicitly picked.
_: {
  name = "Trakt User";
  implementation = "TraktUserImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = 1; # Any
  search_on_add = true;
  minimum_availability = "released";
  list_order = 2;
  limit = 1000;
  auth_user = "\${env.TRAKT_AUTH_USER}";
  trakt_list_type = 0; # User Watch List
  tags = [ "\${ref.tag.watchlist}" ];
}
