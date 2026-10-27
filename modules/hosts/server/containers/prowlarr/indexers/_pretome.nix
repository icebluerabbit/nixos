_: {
  name = "PreToMe";
  implementation = "PreToMe";
  config_contract = "PreToMeSettings";
  # Disabled 2026-08-11: 185 of the 187 PreToMe torrents held were RARed scene
  # releases (99%), i.e. 76% of every archive-bearing torrent in the client.
  # Kept declared rather than removed — configuratarr runs with prune = false
  # (modules/containers/prowlarr.nix:64), so an undeclared indexer would simply
  # stay enabled in Prowlarr's DB. Disabling also 410s this indexer's Torznab
  # feed, hence the matching gate in cross-seed.nix.
  enable = false;
  redirect = false;
  priority = 40;
  protocol = "torrent";
  privacy = "private";
  # Prowlarr's built-in default App Profile (id 1 = "Standard");
  # indexers require app_profile_id > 0.
  app_profile_id = 1;
  tags = [
    "\${ref.tag.movies}"
    "\${ref.tag.tv}"
    "\${ref.tag.music}"
  ];
  fields = {
    username = "\${env.PRETOME_USERNAME}";
    password = "\${env.PRETOME_PASSWORD}";
    pin = "\${env.PRETOME_PIN}";
  };
}
