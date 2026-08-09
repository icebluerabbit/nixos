# containers — named startup waves for the server's nspawn container fleet.
#
# A full-closure switch (e.g. a nixpkgs bump) restarts every container at once;
# the load spike made them miss their start timeout and spiral. Startup is split
# into named barrier targets: a target is reached only once every container that
# declares `before` it has booted, and the next wave's containers (declared
# `after` it) don't start until then.
#
# Only the barrier targets are defined here. Each container declares which wave
# it belongs to in its own host file (modules/hosts/server/containers/<name>.nix)
# via `before`/`after` on these targets.
#
# Wave order:  foundation -> core -> apps -> arr -> downstream
#
# Ordering only (no Requires) — a failed container never deadlocks the next wave.
_: {
  den.aspects.hosts._.server._.containers = {
    aspect-name = "hosts.server.containers";
    nixos.systemd.targets = {
      container-wave-foundation.wantedBy = [ "machines.target" ];
      container-wave-core = {
        wantedBy = [ "machines.target" ];
        after = [ "container-wave-foundation.target" ];
      };
      container-wave-apps = {
        wantedBy = [ "machines.target" ];
        after = [ "container-wave-core.target" ];
      };
      container-wave-arr = {
        wantedBy = [ "machines.target" ];
        after = [ "container-wave-apps.target" ];
      };
      container-wave-downstream = {
        wantedBy = [ "machines.target" ];
        after = [ "container-wave-arr.target" ];
      };
    };
  };
}
