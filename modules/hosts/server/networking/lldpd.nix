# lldpd — Link Layer Discovery Protocol daemon and bridge forwarding
_: {
  den.aspects.hosts._.server._.networking._.lldpd = {
    aspect-name = "hosts.server.networking.lldpd";
    nixos =
      { pkgs, ... }:
      {
        services.lldpd = {
          enable = true;
          extraArgs = [
            "-I"
            "bond0"
          ];
        };

        services.udev.extraRules = ''
          ACTION=="add", SUBSYSTEM=="net", KERNEL=="br-*", RUN+="${pkgs.runtimeShell} -c 'echo 16384 > /sys/class/net/%k/bridge/group_fwd_mask'"
        '';
      };
  };
}
