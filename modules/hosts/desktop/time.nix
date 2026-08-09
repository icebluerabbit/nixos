# time — timezone settings configuration for desktop host
_: {
  den.aspects.hosts._.desktop._.time = {
    aspect-name = "hosts.desktop.time";
    nixos = {
      time.timeZone = "America/New_York";
    };
  };
}
