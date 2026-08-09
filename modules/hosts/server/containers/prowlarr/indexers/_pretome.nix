_: {
  name = "PreToMe";
  implementation = "PreToMe";
  config_contract = "PreToMeSettings";
  enable = true;
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
