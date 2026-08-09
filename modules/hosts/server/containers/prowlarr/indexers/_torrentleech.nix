_: {
  name = "TorrentLeech";
  implementation = "Cardigann";
  config_contract = "CardigannSettings";
  enable = true;
  redirect = false;
  priority = 10;
  protocol = "torrent";
  privacy = "private";
  app_profile_id = 1;
  tags = [
    "\${ref.tag.cloudflare}"
    "\${ref.tag.movies}"
    "\${ref.tag.tv}"
    "\${ref.tag.music}"
  ];
  fields = {
    definitionFile = "torrentleech";
    username = "\${env.TORRENTLEECH_USERNAME}";
    password = "\${env.TORRENTLEECH_PASSWORD}";
  };
}
