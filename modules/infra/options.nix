# modules/infra/options.nix
{ lib, ... }:
let
  vlanSubmodule = lib.types.submodule {
    options = {
      vlanId = lib.mkOption {
        type = lib.types.int;
      };
      subnet = lib.mkOption {
        type = lib.types.str;
        description = "Base subnet IP address (e.g. 10.10.20.0)";
      };
      subnetPrefix = lib.mkOption {
        type = lib.types.int;
        default = 24;
      };
      domain = lib.mkOption {
        type = lib.types.str;
        description = "Subnet domain suffix (e.g. trust.internal)";
      };
      staticAddresses = lib.mkOption {
        type = lib.types.attrsOf lib.types.str;
        default = { };
        description = "Static IP allocations mapped by host/device name on this VLAN";
      };
    };
  };

  clientSubmodule = lib.types.submodule {
    options = {
      mac = lib.mkOption {
        type = lib.types.str;
      };
      name = lib.mkOption {
        type = lib.types.str;
      };
      ip = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
      };
      blocked = lib.mkOption {
        type = lib.types.bool;
        default = false;
      };
      groups = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
      };
      ports = lib.mkOption {
        type = lib.types.attrsOf lib.types.port;
        default = { };
        description = "Named ports exposed by this client";
      };
      proxy = lib.mkOption {
        type = lib.types.nullOr (
          lib.types.submodule {
            options.port = lib.mkOption {
              type = lib.types.str;
              description = "Named port key from `ports` to proxy via Caddy";
            };
          }
        );
        default = null;
      };
    };
  };

  deviceSubmodule = lib.types.submodule {
    options = {
      mac = lib.mkOption {
        type = lib.types.str;
      };
      name = lib.mkOption {
        type = lib.types.str;
      };
      ip = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
      };
      type = lib.mkOption {
        type = lib.types.enum [
          "dhcp"
          "static"
        ];
        default = "dhcp";
      };
    };
  };
in
{
  den.aspects.infra._.options = {
    aspect-name = "infra.options";
    os = _: {
      options.infra = {
        vlans = lib.mkOption {
          type = lib.types.attrsOf vlanSubmodule;
          default = { };
        };
        clients = lib.mkOption {
          type = lib.types.attrsOf clientSubmodule;
          default = { };
        };
        devices = lib.mkOption {
          type = lib.types.attrsOf deviceSubmodule;
          default = { };
        };
      };
    };

    terranix = _: {
      options.infra = {
        vlans = lib.mkOption {
          type = lib.types.attrsOf vlanSubmodule;
          default = { };
        };
        clients = lib.mkOption {
          type = lib.types.attrsOf clientSubmodule;
          default = { };
        };
        devices = lib.mkOption {
          type = lib.types.attrsOf deviceSubmodule;
          default = { };
        };
      };
    };
  };
}
