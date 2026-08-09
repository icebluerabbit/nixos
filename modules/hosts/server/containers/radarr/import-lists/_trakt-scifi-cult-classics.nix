# philrivers' "Best Sci-Fi Cult Classics of All Time" — freeleech-gated, same as
# the other curated Trakt lists. Public user list → TraktListImport.
_: {
  name = "Trakt Sci-Fi Cult Classics";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 12;
  limit = 500;
  username = "philrivers";
  listname = "best-sci-fi-cult-classics-of-all-time";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.scifi-cult-classics}" ];
}
