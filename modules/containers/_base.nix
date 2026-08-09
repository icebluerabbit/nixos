# modules/containers/_base.nix
{
  lib,
  cfg,
  hasPort ? true,
}:
{ config, ... }:
{
  system.stateVersion = "26.05";
  imports = [ ../logging/_vector.nix ];
  logging.vector = cfg.logging;

  networking = {
    enableIPv6 = false;
    useHostResolvConf = lib.mkForce false;
    defaultGateway = lib.mkIf (cfg.gateway != null) cfg.gateway;
    nameservers =
      if (cfg.nameservers != [ ]) then
        cfg.nameservers
      else
        (lib.optional (cfg.gateway != null) cfg.gateway ++ [ "1.1.1.1" ]);
    firewall.allowedTCPPorts = lib.optional hasPort cfg.port;
  };

  services.lldpd = lib.mkIf cfg.lldpd.enable {
    enable = true;
    extraArgs = [
      "-I"
      "eth0"
      "-S"
      "NixOS systemd-nspawn ${config.networking.hostName} container"
    ]
    ++ lib.optionals (cfg.localAddress != null) [
      "-m"
      (lib.head (lib.splitString "/" cfg.localAddress))
    ];
  };

  # lldpd's packaged unit ExecStart references $DAEMON_ARGS (a Debian
  # /etc/default/lldpd slot absent on NixOS) → an "unset variable" warning on
  # every start. Our args ride $LLDPD_OPTIONS (extraArgs above); define
  # DAEMON_ARGS empty to keep the journal clean.
  systemd.services.lldpd.environment.DAEMON_ARGS = lib.mkIf cfg.lldpd.enable "";
}
