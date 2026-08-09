# rps — Receive Packet Steering for container veths.
#
# Container host-side veths (vb-*) are single-queue, so all forwarded traffic
# for a container serializes its RX softirq on one core (~490 Mbit ceiling on the
# vpn-gw → qbittorrent routed path). RPS hashes packets per-flow across all cores.
# Fires on interface creation via udev, so ephemeral container veths are covered
# automatically. Physical NICs are deliberately excluded: they are multi-queue
# (hardware RSS) and already spread across cores — RPS there is redundant.
_: {
  den.aspects.hosts._.server._.networking._.rps = {
    aspect-name = "hosts.server.networking.rps";
    nixos = {
      services.udev.extraRules = ''
        ACTION=="add", SUBSYSTEM=="net", KERNEL=="vb-*", ATTR{queues/rx-0/rps_cpus}="ffffffff"
      '';
    };
  };
}
