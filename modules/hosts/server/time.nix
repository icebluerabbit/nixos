# time — timezone settings configuration for server host
_: {
  den.aspects.hosts._.server._.time = {
    aspect-name = "hosts.server.time";
    nixos = {
      time.timeZone = "America/New_York";
    };
  };
}
