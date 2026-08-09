# justin/imdb-top-rated-movies — freeleech-only wishlist. Monitored (required so
# autobrr pushes are accepted) but the Freeleech-Only profile's min_format_score
# gate blocks any non-freeleech grab. No search-on-add: wait for a freeleech
# announce, never backlog.
_: {
  name = "IMDb Top Rated Movies";
  implementation = "TraktListImport";
  enabled = true;
  enable_auto = true;
  monitor = "movieOnly";
  root_folder_path = "/mnt/media/library/movies";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  search_on_add = false;
  minimum_availability = "released";
  list_order = 3;
  limit = 500;
  username = "justin";
  listname = "imdb-top-rated-movies";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.imdb-top-rated}" ];
}
