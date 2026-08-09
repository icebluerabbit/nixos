# modules/containers/_options.nix
{
  lib,
  defaultPort ? null,
  defaultUid ? null,
  defaultGid ? null,
}:
{
  autoStart = lib.mkOption {
    type = lib.types.bool;
    default = true;
    description = "Whether to start the container automatically on boot";
  };
  hostBridge = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "Bridge interface to attach the container to";
  };
  macAddress = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "MAC address for the container's network interface";
  };
  localAddress = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "IP address with prefix length for the container's network interface";
  };
  gateway = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "Default gateway inside the container";
  };
  bindMounts = lib.mkOption {
    type = lib.types.attrs;
    default = { };
    description = "Bind mounts for the container";
  };
  nameservers = lib.mkOption {
    type = lib.types.listOf lib.types.str;
    default = [ ];
    description = "List of DNS nameservers for the container";
  };
  logging = lib.mkOption {
    type = lib.types.submodule {
      options = import ../logging/_options.nix {
        inherit lib;
        defaultHostType = "container";
      };
    };
    default = { };
  };
  lldpd = lib.mkOption {
    type = lib.types.submodule {
      options = {
        enable = lib.mkOption {
          type = lib.types.bool;
          default = false;
          description = "Enable LLDP daemon inside the container";
        };
      };
    };
    default = { };
  };
  memoryHigh = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = "Soft memory limit for the container cgroup (e.g. \"4G\"). Triggers kernel reclaim without OOM-killing.";
  };
}
// lib.optionalAttrs (defaultPort != null) {
  port = lib.mkOption {
    type = lib.types.port;
    default = defaultPort;
    description = "Service listening port";
  };
}
// lib.optionalAttrs (defaultUid != null) {
  uid = lib.mkOption {
    type = lib.types.int;
    default = defaultUid;
    description = "Static UID for the service user";
  };
}
// lib.optionalAttrs (defaultGid != null) {
  gid = lib.mkOption {
    type = lib.types.int;
    default = defaultGid;
    description = "Static GID for the service group";
  };
}
