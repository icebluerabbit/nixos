_: {
  den.aspects.cli._.ssh = {
    aspect-name = "cli.ssh";
    homeManager = _: {
      programs.ssh = {
        enable = true;
        enableDefaultConfig = false;
      };
    };
  };
}
