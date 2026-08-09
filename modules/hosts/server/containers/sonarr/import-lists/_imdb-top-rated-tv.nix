# justin/imdb-top-rated-tv-shows — freeleech-only wishlist. Monitored (so autobrr
# pushes are accepted) but the Freeleech-Only profile's min_format_score gate
# blocks any non-freeleech grab. No backlog search: wait for a freeleech announce.
_: {
  name = "IMDb Top Rated TV";
  implementation = "TraktListImport";
  enable_automatic_add = true;
  search_for_missing_episodes = false;
  should_monitor = "all";
  root_folder_path = "/mnt/media/library/tv";
  quality_profile_id = "\${ref.quality_profile.Freeleech-Only}";
  series_type = "standard";
  season_folder = true;
  list_order = 2;
  limit = 500;
  username = "justin";
  listname = "imdb-top-rated-tv-shows";
  auth_user = "\${env.TRAKT_AUTH_USER}";
  tags = [ "\${ref.tag.imdb-top-rated}" ];
}
