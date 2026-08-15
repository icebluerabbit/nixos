# modules/infra/options.nix
{ lib, ... }:
let
  # Shared by clients and devices: both plug into a switch port, and the profile
  # on that port is what pairs the two ends of the cable. Produces no Terraform —
  # the switch owns the `port_override` that actually carries the profile.
  portProfileOption = lib.mkOption {
    type = lib.types.nullOr lib.types.str;
    default = null;
    description = ''
      Key into `infra.ports` naming the profile on the upstream switch port this
      thing is wired to. null for anything with no cable — wireless clients,
      container veths, bridges — and for the switch ports' own end of a link.
    '';
  };

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
      name = lib.mkOption {
        type = lib.types.str;
        description = "Display name on the controller (e.g. MEDIA).";
      };
      isolated = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = "Whether the controller blocks traffic to other local networks.";
      };
      dnsServers = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        description = "Resolvers handed out by DHCP on this VLAN.";
      };
      staticAddresses = lib.mkOption {
        type = lib.types.attrsOf lib.types.str;
        default = { };
        description = "Static IP allocations mapped by host/device name on this VLAN";
      };
      dhcpStart = lib.mkOption {
        type = lib.types.str;
        description = "First address of the DHCP pool. Static allocations must stay below it.";
      };
      dhcpStop = lib.mkOption {
        type = lib.types.str;
        description = "Last address of the DHCP pool.";
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
      vlan = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = ''
          VLAN this client sits on. Stated rather than inferred: matching `ip`
          against a subnet is guesswork, and a client can have no address at all.
          null only for something genuinely not on one VLAN, like a trunk.
        '';
      };
      kind = lib.mkOption {
        type = lib.types.enum [
          "virtual"
          "physical"
        ];
        default = "physical";
        description = ''
          Whether this MAC belongs to real hardware or to a software interface.
          `physical` is a NIC — a device, a host's own port, a BMC. `virtual` is
          anything a host synthesises: container veths, VLAN subinterfaces,
          bridges, bonds.

          Deliberately not finer-grained: container vs bridge vs subinterface is
          already stated by the host's own config, and duplicating it here would
          give two sources that can disagree.
        '';
      };
      portProfile = portProfileOption;
      icon = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "Icon name for topology diagrams and dashboard cards.";
      };
      description = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = "One-line summary of what this client is for.";
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
      portProfile = portProfileOption;
      ports = lib.mkOption {
        type = lib.types.attrsOf (
          lib.types.submodule {
            options = {
              name = lib.mkOption {
                type = lib.types.str;
                default = "";
                description = "Label on the port, as shown on the controller.";
              };
              profile = lib.mkOption {
                type = lib.types.nullOr lib.types.str;
                default = null;
                description = ''
                  Key into `infra.ports` for the profile on this port — the same
                  name whatever is plugged in puts in its `portProfile`. That
                  pairing is the cable. null for a port carrying no profile.
                '';
              };
              aggregateMembers = lib.mkOption {
                type = lib.types.listOf lib.types.int;
                default = [ ];
                description = "Port indices forming a LAG, set only on the lead port.";
              };
            };
          }
        );
        default = { };
        description = ''
          Switch ports, keyed by index. Derived read-only from
          `extra.port_override` — that stays the source and reaches Terraform
          verbatim. Exists because a port profile identifies a cable but not
          which switch port terminates it, which a physical topology needs.
        '';
      };
    };
  };
  portSubmodule = lib.types.submodule {
    options = {
      name = lib.mkOption {
        type = lib.types.str;
      };
      network = lib.mkOption {
        type = lib.types.str;
        description = "Network carried untagged on ports using this profile.";
      };
      unifiId = lib.mkOption {
        type = lib.types.nullOr lib.types.str;
        default = null;
        description = ''
          Controller object id — these profiles have no stable human key. null
          for a profile that has never been applied, and so has nothing to import.
        '';
      };
      resourceName = lib.mkOption {
        type = lib.types.str;
        description = "Terraform address for this profile. Referenced by device port overrides.";
      };
      trunk = lib.mkOption {
        type = lib.types.bool;
        default = false;
        description = "Whether the profile carries tagged VLANs beyond its native one.";
      };
      excludedNetworks = lib.mkOption {
        type = lib.types.listOf lib.types.str;
        default = [ ];
        description = "Networks kept off a trunk. Empty on an access port.";
      };
    };
  };

  wanSubmodule = lib.types.submodule {
    options = {
      name = lib.mkOption {
        type = lib.types.str;
      };
      type = lib.mkOption {
        type = lib.types.str;
        default = "dhcp";
      };
      uploadKbps = lib.mkOption {
        type = lib.types.int;
      };
      downloadKbps = lib.mkOption {
        type = lib.types.int;
      };
      unifiId = lib.mkOption {
        type = lib.types.str;
      };
    };
  };

  wlanSubmodule = lib.types.submodule {
    options = {
      ssid = lib.mkOption {
        type = lib.types.str;
      };
      network = lib.mkOption {
        type = lib.types.str;
        description = "Network clients on this SSID land on.";
      };
      bands = lib.mkOption {
        type = lib.types.listOf lib.types.str;
      };
      wpa3 = lib.mkOption {
        type = lib.types.bool;
        default = true;
      };
      hidden = lib.mkOption {
        type = lib.types.bool;
        default = false;
      };
      unifiId = lib.mkOption {
        type = lib.types.str;
      };
    };
  };

  vpnSubmodule = lib.types.submodule {
    options = {
      name = lib.mkOption {
        type = lib.types.str;
      };
      subnet = lib.mkOption {
        type = lib.types.str;
      };
      port = lib.mkOption {
        type = lib.types.port;
      };
      wanInterface = lib.mkOption {
        type = lib.types.str;
        default = "wan";
      };
      unifiId = lib.mkOption {
        type = lib.types.str;
      };
    };
  };
in
{
  den.aspects.infra._.options = {
    aspect-name = "infra.options";
    os =
      { config, ... }:
      {
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
          ports = lib.mkOption {
            type = lib.types.attrsOf portSubmodule;
            default = { };
          };
          wans = lib.mkOption {
            type = lib.types.attrsOf wanSubmodule;
            default = { };
          };
          wlans = lib.mkOption {
            type = lib.types.attrsOf wlanSubmodule;
            default = { };
          };
          vpns = lib.mkOption {
            type = lib.types.attrsOf vpnSubmodule;
            default = { };
          };
        };

        # A client or device names the profile on the switch port it is wired to,
        # and the switch names the same profile in its `port_override`. Nothing in
        # Terraform ties the two ends together, so the pairing is only as good as
        # these two checks.
        config.assertions =
          let
            declared = lib.filterAttrs (_: v: v.portProfile != null);
            missing = lib.mapAttrsToList (
              name: v: {
                assertion = config.infra.ports ? ${v.portProfile};
                message = "infra: ${name} declares portProfile \"${v.portProfile}\", which is not in infra.ports";
              }
            );
          in
          missing (declared config.infra.clients)
          ++ missing (declared config.infra.devices)
          ++ lib.mapAttrsToList (name: v: {
            # Access ports carry exactly one untagged network, so a client on one
            # must be on that network. Trunks carry many and say nothing here.
            assertion =
              let
                p = config.infra.ports.${v.portProfile} or null;
              in
              p == null || p.trunk || v.vlan == p.network;
            message = "infra: client ${name} is on vlan ${toString v.vlan} but its port profile ${v.portProfile} lands untagged on ${
              config.infra.ports.${v.portProfile}.network
            }";
          }) (declared config.infra.clients);
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
        ports = lib.mkOption {
          type = lib.types.attrsOf portSubmodule;
          default = { };
        };
        wans = lib.mkOption {
          type = lib.types.attrsOf wanSubmodule;
          default = { };
        };
        wlans = lib.mkOption {
          type = lib.types.attrsOf wlanSubmodule;
          default = { };
        };
        vpns = lib.mkOption {
          type = lib.types.attrsOf vpnSubmodule;
          default = { };
        };
      };
    };
  };
}
