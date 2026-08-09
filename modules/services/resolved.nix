/*
  systemd-resolved — local DNS stub resolver and DNSSEC validator; provides /etc/resolv.conf via a 127.0.0.53 stub listener
  https://www.freedesktop.org/software/systemd/man/systemd-resolved.service.html
*/
_: {
  den.aspects.services._.resolved = {
    aspect-name = "services.resolved";
    nixos = {
      services.resolved = {
        enable = true;
        settings.Resolve = {
          DNSSEC = "no";
          Cache = "no-negative";
        };
      };
    };
  };
}
