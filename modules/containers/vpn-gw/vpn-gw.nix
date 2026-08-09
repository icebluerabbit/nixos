_: {
  den.aspects.containers._.vpn-gw = {
    aspect-name = "containers.vpn-gw";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs."vpn-gw";
      in
      {
        options.containerConfigs."vpn-gw" = (import ../_options.nix { inherit lib; }) // {
          # vpn-gw is a TUNNEL BROKER. It brings up a pool of WireGuard tunnels and, via a
          # small reactive control daemon (broker.py), allocates them to clients on demand:
          # a client claims a tunnel, the broker leases its forwarded port (provider-specific)
          # and installs the inbound DNAT to that client. No client embeds tunnel-selection
          # logic anymore — the broker owns the pool, health, and port-forward mechanism.
          #
          #   egress : traffic arriving on the tunnel's veth (`iif gw-vpnN`) is policy-routed
          #            out the tunnel (`lookup <table>`; the conf's `Table=<n>` installs
          #            `default dev wgN` there) and masqueraded.
          #   ingress: catch-all DNAT of everything inbound on `wgN` → the CURRENT owner's IP,
          #            via the mutable `dnat_map` the broker populates at claim/dnat time.
          tunnels = lib.mkOption {
            type = lib.types.listOf (
              lib.types.submodule {
                options = {
                  name = lib.mkOption {
                    type = lib.types.str;
                    description = "wg interface name (== bind-mounted conf filename, e.g. wg1)";
                  };
                  conf = lib.mkOption {
                    type = lib.types.str;
                    description = "Host path to the sops-decrypted wg-quick ini";
                  };
                  bridge = lib.mkOption {
                    type = lib.types.str;
                    description = "Per-tunnel host bridge for the p2p veth (br-vpnN); veth == br-* stripped";
                  };
                  ip = lib.mkOption {
                    type = lib.types.str;
                    description = "vpn-gw address on the veth (no mask)";
                  };
                  table = lib.mkOption {
                    type = lib.types.int;
                    description = "Routing-table id for this tunnel (must match Table= in its conf)";
                  };
                  # Provider port-forward method. The broker dispatches on `type`:
                  #   natmap → natpmpc against `gateway` (Proton; re-leased on each renew)
                  #   static → fixed `port` (AirVPN portal; renew is a no-op)
                  #   none   → egress-only; dnat() is rejected
                  portForward = lib.mkOption {
                    type = lib.types.submodule {
                      options = {
                        type = lib.mkOption {
                          type = lib.types.enum [
                            "natmap"
                            "static"
                            "none"
                          ];
                          description = "Port-forward mechanism for this tunnel's provider";
                        };
                        gateway = lib.mkOption {
                          type = lib.types.nullOr lib.types.str;
                          default = null;
                          description = "natmap: provider NAT-PMP gateway (reached through the tunnel)";
                        };
                        port = lib.mkOption {
                          type = lib.types.nullOr lib.types.port;
                          default = null;
                          description = "static: fixed portal-assigned forwarded port";
                        };
                      };
                    };
                    description = "Per-tunnel provider port-forward method";
                  };
                };
              }
            );
            default = [ ];
            description = "WireGuard tunnel pool";
          };

          controlPort = lib.mkOption {
            type = lib.types.port;
            default = 9099;
            description = "Broker HTTP control port (firewalled to client IPs + localhost)";
          };
          ttlSeconds = lib.mkOption {
            type = lib.types.int;
            default = 120;
            description = "Claim TTL — a claim not renewed within this window is reaped (must exceed client renew cadence)";
          };
          failThreshold = lib.mkOption {
            type = lib.types.int;
            default = 3;
            description = "Consecutive health-fails before a tunnel is marked unhealthy + its claim voided";
          };
          flapWindowSeconds = lib.mkOption {
            type = lib.types.int;
            default = 86400;
            description = "Rolling window over which tunnel down-transitions are counted into a flap score (soft claim de-prioritization). In-memory; resets on broker/container restart";
          };
          excludeSeconds = lib.mkOption {
            type = lib.types.int;
            default = 120;
            description = "Lifetime of a forced-rotation exclusion (POST /rotate): how long a client is steered off the tunnel it just held before its saved preference can re-grab it. Must exceed the client keepalive cadence so the next tick lands elsewhere. In-memory; NOT a health signal";
          };

          nodeExporterPort = lib.mkOption {
            type = lib.types.port;
            default = 9100;
          };
          wireguardExporterPort = lib.mkOption {
            type = lib.types.port;
            default = 9586;
          };
        };

        config =
          let
            inherit (cfg) tunnels;
            # vpn-gw's side of each p2p veth. Prefixed so it doesn't collide with the
            # client's host-side veth on the same bridge (nixos-container names the
            # host veth exactly the key, no per-container prefix). br-vpn1 -> gw-vpn1.
            vethOf = t: "gw-" + lib.removePrefix "br-" t.bridge;
            tunNames = map (t: t.name) tunnels;
            vethNames = map vethOf tunnels;
            nftSet = xs: "{ " + lib.concatMapStringsSep ", " (s: ''"${s}"'') xs + " }";
          in
          {
            containers.vpn-gw = {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;

              # main eth0 stays on the download VLAN (own traffic + reaching wg endpoints);
              # one p2p veth per tunnel into its br-vpnN bridge, joining the client.
              extraVeths = lib.listToAttrs (
                map (
                  t:
                  lib.nameValuePair (vethOf t) {
                    hostBridge = t.bridge;
                    localAddress = "${t.ip}/24";
                  }
                ) tunnels
              );

              # sops-rendered confs bind-mounted at /run/secrets/<name>.conf (wg-quick
              # derives the iface name from the filename), merged with any extra mounts.
              bindMounts =
                cfg.bindMounts
                // lib.listToAttrs (
                  map (
                    t:
                    lib.nameValuePair "/run/secrets/${t.name}.conf" {
                      hostPath = t.conf;
                      isReadOnly = true;
                    }
                  ) tunnels
                );

              additionalCapabilities = [
                "CAP_NET_ADMIN"
                "CAP_NET_RAW"
              ];
              allowedDevices = [
                {
                  node = "/dev/net/tun";
                  modifier = "rwm";
                }
              ];

              config =
                { pkgs, lib, ... }:
                let
                  pythonEnv = pkgs.python3.withPackages (ps: [ ps.flask ]);
                  brokerConfig = pkgs.writeText "vpn-broker.json" (
                    builtins.toJSON {
                      control_port = cfg.controlPort;
                      listen_ip = "0.0.0.0";
                      ttl_seconds = cfg.ttlSeconds;
                      fail_threshold = cfg.failThreshold;
                      flap_window_seconds = cfg.flapWindowSeconds;
                      exclude_seconds = cfg.excludeSeconds;
                      tunnels = lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair t.name {
                            port_forward = lib.filterAttrs (_: v: v != null) t.portForward;
                          }
                        ) tunnels
                      );
                    }
                  );
                  ctl = toString cfg.controlPort;
                in
                {
                  imports = [
                    (import ../_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  # AirVPN/Proton WG confs carry an IPv6 Address; _base disables IPv6 —
                  # re-enable so wg-quick's `ip -6 addr add` succeeds (v6 only on wg ifaces).
                  networking.enableIPv6 = lib.mkForce true;

                  boot.kernel.sysctl = {
                    "net.ipv4.ip_forward" = 1;
                    "net.ipv4.conf.all.rp_filter" = 0;
                  };

                  networking = {
                    firewall.enable = false;
                    nftables = {
                      enable = true;
                      ruleset = ''
                        table ip nat {
                          # populated at runtime by the broker: iifname wgN -> owning client IP
                          map dnat_map { type ifname : ipv4_addr; }
                          chain prerouting {
                            type nat hook prerouting priority dstnat;
                            iifname ${nftSet tunNames} meta l4proto { tcp, udp } dnat to iifname map @dnat_map
                          }
                          chain postrouting {
                            type nat hook postrouting priority srcnat;
                            oifname ${nftSet tunNames} masquerade
                          }
                        }
                        table ip filter {
                          chain forward {
                            type filter hook forward priority 0;
                            tcp flags syn tcp option maxseg size set rt mtu
                            ct state established,related accept
                            iifname ${nftSet vethNames} oifname ${nftSet tunNames} accept
                            iifname ${nftSet tunNames} oifname ${nftSet vethNames} accept
                            drop
                          }
                        }
                      '';
                    };
                  };

                  environment.systemPackages = [
                    pkgs.wireguard-tools
                    pkgs.iproute2
                  ];

                  systemd = {
                    services = lib.mkMerge [
                      # ── per-tunnel bring-up: wg-quick reads the conf (its `Table=<n>`
                      #    installs `default dev wgN` in table n). MTU bumped 1320→1420. ──
                      (lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair "wg-${t.name}" {
                            description = "WireGuard tunnel ${t.name}";
                            wantedBy = [ "multi-user.target" ];
                            after = [ "network-online.target" ];
                            wants = [ "network-online.target" ];
                            before = [ "vpn-routing-${t.name}.service" ];
                            path = [
                              pkgs.wireguard-tools
                              pkgs.iproute2
                            ];
                            serviceConfig = {
                              Type = "oneshot";
                              RemainAfterExit = true;
                              # Drop any leftover link before `up`. Without this the unit
                              # can wedge permanently: if one `wg-quick up` fails with
                              # "`wgN' already exists" the unit is inactive, so systemd
                              # never runs ExecStop on the next attempt — the orphan link
                              # survives, every retry hits the same error, and Restart=
                              # hammers it every 10s forever while the orphan sends
                              # handshakes nothing manages. `-` = ignore "does not exist"
                              # on the normal path, where there is no link to delete.
                              ExecStartPre = "-${pkgs.iproute2}/bin/ip link delete dev ${t.name}";
                              ExecStart = "${pkgs.wireguard-tools}/bin/wg-quick up /run/secrets/${t.name}.conf";
                              # `-` = ignore failure (never fail the unit / trigger the loop)
                              ExecStartPost = "-${pkgs.iproute2}/bin/ip link set dev ${t.name} mtu 1420";
                              # `-` too: a failed `down` must not leave the unit failed with
                              # the link still present — that is the wedge above.
                              ExecStop = "-${pkgs.wireguard-tools}/bin/wg-quick down /run/secrets/${t.name}.conf";
                              Restart = "on-failure";
                              RestartSec = "10s";
                            };
                          }
                        ) tunnels
                      ))

                      # ── per-tunnel iif policy routing: traffic in on gw-vpnN → table 20N
                      #    → out wgN. One unit per tunnel (wants/after its own wgN only) so a
                      #    dead tunnel can't block the others; del-then-add = idempotent. ──
                      (lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair "vpn-routing-${t.name}" {
                            description = "Policy routing for ${t.name} (iif ${vethOf t} → table ${toString t.table})";
                            wantedBy = [ "multi-user.target" ];
                            after = [ "wg-${t.name}.service" ];
                            wants = [ "wg-${t.name}.service" ];
                            path = [ pkgs.iproute2 ];
                            serviceConfig = {
                              Type = "oneshot";
                              RemainAfterExit = true;
                            };
                            script = ''
                              ip rule del iif ${vethOf t} lookup ${toString t.table} 2>/dev/null || true
                              ip rule add iif ${vethOf t} lookup ${toString t.table} priority 200
                            '';
                          }
                        ) tunnels
                      ))

                      {
                        # ── nftables flow offload for all tunnel veths + wg ifaces. Single
                        #    shared flowtable; best-effort (|| true) so it can never abort
                        #    forwarding — the declarative ruleset is the source of truth. ──
                        flow-offload = {
                          description = "nftables flow offload for tunnel veths + wg interfaces";
                          wantedBy = [ "multi-user.target" ];
                          after = map (t: "vpn-routing-${t.name}.service") tunnels;
                          path = [ pkgs.nftables ];
                          serviceConfig = {
                            Type = "oneshot";
                            RemainAfterExit = true;
                          };
                          script = ''
                            devs="${lib.concatStringsSep ", " (vethNames ++ tunNames)}"
                            nft add flowtable ip filter f "{ hook ingress priority filter; devices = { $devs }; }" || true
                            nft insert rule ip filter forward ct state established,related flow add @f || true
                          '';
                        };

                        # ── the tunnel broker: allocation registry + health + port-forward +
                        #    dynamic DNAT. Reactive (no poll loop); health is pushed in by the
                        #    per-tunnel probe timers below, TTL enforced by the reap timer. ──
                        vpn-broker = {
                          description = "vpn-gw tunnel broker (claim/dnat/renew + health + DNAT)";
                          wantedBy = [ "multi-user.target" ];
                          after = [
                            "nftables.service"
                            "network-online.target"
                          ];
                          wants = [ "network-online.target" ];
                          environment = {
                            BROKER_CONFIG = "${brokerConfig}";
                            BROKER_NFT = "${pkgs.nftables}/bin/nft";
                            BROKER_NATPMPC = lib.getExe' pkgs.libnatpmp "natpmpc";
                            BROKER_IP = "${pkgs.iproute2}/bin/ip";
                          };
                          serviceConfig = {
                            ExecStart = "${pythonEnv}/bin/python3 ${./broker.py}";
                            Restart = "on-failure";
                            RestartSec = "5s";
                          };
                        };

                        # ── TTL reaper: poke /reap so abandoned claims (dead clients that
                        #    never released) return to the pool. ──
                        vpn-reap = {
                          description = "Reap expired broker claims";
                          after = [ "vpn-broker.service" ];
                          path = [ pkgs.curl ];
                          serviceConfig.Type = "oneshot";
                          script = ''
                            curl -fsS -m 5 -X POST http://127.0.0.1:${ctl}/reap >/dev/null 2>&1 || true
                          '';
                        };
                      }

                      # ── per-tunnel health probe: curl the exit IP THROUGH wgN (its Table=20N
                      #    routes the default out that iface) and PUSH the result to the broker,
                      #    which folds it (with any inline natpmp failures) into the tunnel's
                      #    health + caches exit_ip for dnat/renew replies. ──
                      (lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair "tunnel-health-${t.name}" {
                            description = "Probe ${t.name} exit IP → push health to the broker";
                            after = [
                              "vpn-routing-${t.name}.service"
                              "vpn-broker.service"
                            ];
                            path = [
                              pkgs.curl
                              pkgs.iputils
                              pkgs.gawk
                            ];
                            serviceConfig.Type = "oneshot";
                            script = ''
                              ip=$(curl --interface ${t.name} -4 -fsS --max-time 8 https://api.ipify.org 2>/dev/null || true)
                              if [ -n "$ip" ]; then
                                # tunnel RTT: ICMP to 1.1.1.1 bound to this wg iface (Proton's gw
                                # doesn't answer ping). Parse avg from `rtt min/avg/max/mdev = …`.
                                # Empty on ping failure → JSON null (broker keeps last known).
                                lat=$(ping -I ${t.name} -n -c 3 -w 5 1.1.1.1 2>/dev/null \
                                  | awk -F'/' '/^rtt/{print $5}')
                                [ -z "$lat" ] && lat=null
                                curl -fsS -m 5 -X POST http://127.0.0.1:${ctl}/health \
                                  -H 'Content-Type: application/json' \
                                  -d "{\"tunnel\":\"${t.name}\",\"up\":true,\"exit_ip\":\"$ip\",\"latency\":$lat}" >/dev/null 2>&1 || true
                              else
                                curl -fsS -m 5 -X POST http://127.0.0.1:${ctl}/health \
                                  -H 'Content-Type: application/json' \
                                  -d "{\"tunnel\":\"${t.name}\",\"up\":false}" >/dev/null 2>&1 || true
                              fi
                            '';
                          }
                        ) tunnels
                      ))

                      # ── per-tunnel handshake watchdog: ExecCondition passes (exit 0) only
                      #    when the handshake is missing/stale (>180s); then ExecStart restarts
                      #    the tunnel (down+up via wg-quick) to force a fresh handshake. Healthy
                      #    → ExecCondition exits 1 → ExecStart skipped, unit not failed. Driven
                      #    by the per-tunnel timer below. Restart stays timer-driven; the broker
                      #    only voids claims on unhealthy, it does not restart tunnels. ──
                      (lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair "wg-watchdog-${t.name}" {
                            description = "Restart ${t.name} when its WireGuard handshake goes stale";
                            after = [ "vpn-routing-${t.name}.service" ];
                            serviceConfig = {
                              Type = "oneshot";
                              ExecCondition = pkgs.writeShellScript "wg-stale-${t.name}" ''
                                hs=$(${pkgs.wireguard-tools}/bin/wg show ${t.name} latest-handshakes | ${pkgs.coreutils}/bin/cut -f2)
                                now=$(${pkgs.coreutils}/bin/date +%s)
                                [ -z "$hs" ] || [ "$hs" = 0 ] || [ $((now - hs)) -gt 180 ]
                              '';
                              ExecStart = "${pkgs.systemd}/bin/systemctl restart wg-${t.name}.service";
                            };
                          }
                        ) tunnels
                      ))
                    ];

                    timers = lib.mkMerge [
                      (lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair "tunnel-health-${t.name}" {
                            description = "Periodic ${t.name} health probe → broker";
                            wantedBy = [ "timers.target" ];
                            timerConfig = {
                              OnBootSec = "45s";
                              OnUnitActiveSec = "30s";
                            };
                          }
                        ) tunnels
                      ))
                      {
                        vpn-reap = {
                          description = "Periodic broker claim reap";
                          wantedBy = [ "timers.target" ];
                          timerConfig = {
                            OnBootSec = "90s";
                            OnUnitActiveSec = "30s";
                          };
                        };
                      }
                      (lib.listToAttrs (
                        map (
                          t:
                          lib.nameValuePair "wg-watchdog-${t.name}" {
                            description = "Periodic ${t.name} handshake watchdog";
                            wantedBy = [ "timers.target" ];
                            timerConfig = {
                              OnBootSec = "120s";
                              OnUnitActiveSec = "60s";
                            };
                          }
                        ) tunnels
                      ))
                    ];
                  };

                  # metrics: node (host/net) + per-tunnel wireguard handshake/transfer.
                  # Tunnel allocation/health/port/exit-ip now come from the broker's /export.
                  services.prometheus.exporters.node = {
                    enable = true;
                    listenAddress = "127.0.0.1";
                    port = cfg.nodeExporterPort;
                    enabledCollectors = [
                      "conntrack"
                      "netdev"
                      "netstat"
                      "systemd"
                    ];
                  };
                  services.prometheus.exporters.wireguard = {
                    enable = true;
                    listenAddress = "127.0.0.1";
                    port = cfg.wireguardExporterPort;
                  };
                };
            };
          };
      };
  };
}
