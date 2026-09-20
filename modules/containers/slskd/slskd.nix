/*
  slskd — headless Soulseek client (web UI + API), routed through the vpn-gw tunnel broker
  https://github.com/slskd/slskd
*/
_: {
  den.aspects.containers._.slskd = {
    aspect-name = "containers.slskd";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.slskd =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 5030; # web UI (LAN); opened by _base
            defaultUid = 5034;
            defaultGid = 5034;
          })
          // {
            broker = lib.mkOption {
              type = lib.types.str;
              description = "Base URL of the vpn-gw tunnel broker (http://<vpn-gw-ip>:<control-port>)";
            };
            envFile = lib.mkOption {
              type = lib.types.path;
              description = "Env file (sops) with SLSKD_SLSK_USERNAME/PASSWORD + SLSKD_USERNAME/PASSWORD";
            };
            incompleteDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/slskd/incomplete";
              description = "Local incomplete-download dir (under the state dir; off NFS, and not shadowed by the unit's PrivateTmp like /var/tmp would be)";
            };
            downloadDir = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/complete/soulseek";
              description = "Completed downloads dir (same fs as the library → hardlink imports)";
            };
            shares = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "Shared directories advertised to the Soulseek network";
            };

            # ── downloadDir housekeeping (soulseek-reap) ──
            reapEmptyMinutes = lib.mkOption {
              type = lib.types.int;
              default = 120;
              description = "Age (mtime, minutes) an empty album dir in downloadDir must reach before removal. Gates against a multi-file album whose remaining tracks are still landing.";
            };
            reapFailedImportsDays = lib.mkOption {
              type = lib.types.int;
              default = 21;
              description = "Age (mtime, days) a failed_imports/ entry must reach before deletion. soularr denylists what it moves there, so nothing ever retries it.";
            };

            # ── search history pruning (slskd-search-prune) ──
            searchRetentionDays = lib.mkOption {
              type = lib.types.int;
              default = 7;
              description = "Age (startedAt, days) a stored search must reach before deletion. slskd has retention for transfers/files/logs/share-cache but none for searches, so search.db grows without bound.";
            };
            listenPortSeed = lib.mkOption {
              type = lib.types.port;
              default = 50300;
              description = "Initial Soulseek listen port; the broker PATCHes the live forwarded port at runtime";
            };

            # ── performance/policy tuning ── conservative defaults here; the host sets the
            # values it wants for its hardware (mirrors the qbittorrent aspect's knobs).
            childLimit = lib.mkOption {
              type = lib.types.int;
              default = 32;
              description = "Distributed-network child limit (search fan-out). Higher reaches more searchers = more upload demand, at more CPU/connections.";
            };
            connectTimeout = lib.mkOption {
              type = lib.types.int;
              default = 10000;
              description = "Peer connect timeout (ms).";
            };
            inactivityTimeout = lib.mkOption {
              type = lib.types.int;
              default = 15000;
              description = "Peer inactivity timeout (ms).";
            };
            transferTimeout = lib.mkOption {
              type = lib.types.int;
              default = 30000;
              description = "Transfer stall timeout (ms).";
            };
            readBuffer = lib.mkOption {
              type = lib.types.int;
              default = 16384;
              description = "Per-connection read buffer (bytes).";
            };
            writeBuffer = lib.mkOption {
              type = lib.types.int;
              default = 16384;
              description = "Per-connection write buffer (bytes).";
            };
            transferBuffer = lib.mkOption {
              type = lib.types.int;
              default = 262144;
              description = "Transfer buffer (bytes). Large saturates fast peers/tunnel, costs RAM per active transfer.";
            };
            writeQueue = lib.mkOption {
              type = lib.types.int;
              default = 250;
              description = "Queued writes per transfer.";
            };
            shareCacheMode = lib.mkOption {
              type = lib.types.enum [
                "memory"
                "sqlite"
              ];
              default = "sqlite";
              description = "Share index storage. memory = fast searches at the cost of RAM; sqlite = on-disk.";
            };
            shareCacheWorkers = lib.mkOption {
              type = lib.types.int;
              default = 8;
              description = "Parallel share-scan workers (scale to core count).";
            };
            uploadSlots = lib.mkOption {
              type = lib.types.int;
              default = 20;
              description = "Concurrent upload slots. More = serve more peers at once = more seeding.";
            };
            uploadSpeedLimit = lib.mkOption {
              type = lib.types.int;
              default = 2147483647;
              description = "Upload speed cap (bytes/s). 2147483647 = unlimited (0 would mean zero).";
            };
            downloadSlots = lib.mkOption {
              type = lib.types.int;
              default = 2147483647;
              description = "Concurrent download slots. 2147483647 = unlimited.";
            };
            downloadSpeedLimit = lib.mkOption {
              type = lib.types.int;
              default = 2147483647;
              description = "Download speed cap (bytes/s). 2147483647 = unlimited.";
            };
            searchConcurrency = lib.mkOption {
              type = lib.types.int;
              default = 25;
              description = "Concurrent incoming-search evaluations.";
            };
            searchCircuitBreaker = lib.mkOption {
              type = lib.types.int;
              default = 1000;
              description = "Max queued incoming searches before shedding (raise with searchConcurrency).";
            };
            searchResponseFileLimit = lib.mkOption {
              type = lib.types.int;
              default = 1000;
              description = "Max files returned per incoming-search response.";
            };
            leecherUploadPriority = lib.mkOption {
              type = lib.types.int;
              default = 100;
              description = "Upload priority for the leechers group (served after real sharers).";
            };

            vpnPool = lib.mkOption {
              type = lib.types.listOf (
                lib.types.submodule {
                  options = {
                    tunnel = lib.mkOption {
                      type = lib.types.str;
                      description = "Broker tunnel name this veth is wired to (e.g. wg1)";
                    };
                    veth = lib.mkOption {
                      type = lib.types.str;
                      description = "Local interface name for this tunnel's p2p veth (e.g. vpn1)";
                    };
                    ip = lib.mkOption {
                      type = lib.types.str;
                      description = "This client's address on the veth (broker DNAT target)";
                    };
                    gateway = lib.mkOption {
                      type = lib.types.str;
                      description = "vpn-gw's address on the veth (egress next-hop for the default route)";
                    };
                    bridge = lib.mkOption {
                      type = lib.types.str;
                      description = "Host bridge for this veth (br-vpnN)";
                    };
                  };
                }
              );
              default = [ ];
              description = "VPN tunnel pool wiring (decoupled from vpn-gw); the broker selects among these";
            };
            apiPort = lib.mkOption {
              type = lib.types.port;
              default = 9910;
              description = ''
                Port for managarr-slskd, the sidecar serving this service's
                records to managarr.

                9910 on every managarr-* sidecar. Each container has its own
                IP, so one number is unambiguous everywhere and there is no
                per-service arithmetic to get wrong.
              '';
            };
            managarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                managarr base URL the sidecar relays events to; empty disables
                relaying and leaves the read half working.
              '';
            };
          };

        config.containers.slskd =
          let
            # Resolved in the HOST's package set. pkgs inside a container
            # body is the container's own and carries no overlays, so
            # pkgs.homelab there fails with "attribute 'homelab' missing".
            # The store path is what crosses the boundary.
            managarrSidecarExe = lib.getExe pkgs.homelab.managarr-slskd;
            cfg = config.containerConfigs.slskd;
            # Resolved from host pkgs so the homelab overlay is in scope; the
            # store path pulls into the container closure via ExecStart.
            brokerClient = lib.getExe pkgs.homelab.vpn-broker-client-slskd;
            pruneClient = lib.getExe pkgs.homelab.slskd-prune;
          in
          {
            inherit (cfg) autoStart;
            ephemeral = true;
            timeoutStartSec = "5min";
            privateNetwork = true;
            inherit (cfg) hostBridge;
            localMacAddress = cfg.macAddress;
            inherit (cfg) localAddress bindMounts;

            # one p2p veth per pool tunnel into its br-vpnN bridge (warm for instant failover)
            extraVeths = lib.listToAttrs (
              map (
                v:
                lib.nameValuePair v.veth {
                  hostBridge = v.bridge;
                  localAddress = "${v.ip}/24";
                }
              ) cfg.vpnPool
            );

            additionalCapabilities = [ "CAP_NET_ADMIN" ]; # broker client sets the default route

            config =
              { pkgs, lib, ... }:
              let
                brokerCfg = pkgs.writeText "slskd-broker.json" (
                  builtins.toJSON {
                    inherit (cfg) broker;
                    api = "http://localhost:${toString cfg.port}";
                    client_name = "slskd";
                    lan_cidr = "10.10.0.0/16";
                    lan_gateway = cfg.gateway;
                    lan_dev = "eth0";
                    ip_bin = "${pkgs.iproute2}/bin/ip";
                    # Persistent dir (/var/lib/slskd is bind-mounted → survives the ephemeral
                    # container) for cross-restart state: last-held tunnel (tunnel-preference).
                    state_dir = "/var/lib/slskd/.vpnbroker";
                    pool = lib.listToAttrs (
                      map (
                        v:
                        lib.nameValuePair v.tunnel {
                          inherit (v) veth ip gateway;
                        }
                      ) cfg.vpnPool
                    );
                  }
                );
                inherit brokerClient;
              in
              {
                imports = [
                  {
                    networking.firewall.allowedTCPPorts = [ cfg.apiPort ];

                    systemd.services.managarr-slskd = {
                      description = "Serve slskd's event log to managarr";
                      after = [ "slskd.service" ];
                      wantedBy = [ "multi-user.target" ];
                      wants = [ "slskd.service" ];

                      script =
                        "exec "
                        + lib.concatStringsSep " " (
                          [
                            managarrSidecarExe
                            "--addr :${toString cfg.apiPort}"
                            "--metrics.addr 127.0.0.1:9909"
                          ]
                          ++ lib.optional (cfg.managarrUrl != "") "--managarr-url ${cfg.managarrUrl}"
                          ++ [ "--upstream-url http://127.0.0.1:5030" ]
                        );

                      serviceConfig = {
                        User = "slskd";
                        Group = "slskd";
                        Restart = "on-failure";
                        RestartSec = 5;
                        ProtectSystem = "strict";
                        ProtectHome = true;
                        PrivateDevices = true;
                        PrivateTmp = true;
                        NoNewPrivileges = true;
                        RestrictAddressFamilies = [
                          "AF_INET"
                          "AF_INET6"
                        ];
                      };
                    };
                  }
                  (import ../_base.nix { inherit lib cfg; })
                ]; # opens cfg.port (web) on LAN

                # asymmetric-ish paths (inbound via DNAT'd veth, egress via default veth) — like
                # vpn-gw/qbt, disable strict reverse-path filtering.
                boot.kernel.sysctl."net.ipv4.conf.all.rp_filter" = 0;

                users.users.slskd = {
                  uid = lib.mkForce cfg.uid;
                  group = "slskd";
                  isSystemUser = true;
                };
                users.groups.slskd.gid = lib.mkForce cfg.gid;

                services.slskd = {
                  enable = true;
                  openFirewall = false;
                  user = "slskd";
                  group = "slskd";
                  domain = null; # no built-in nginx — Caddy fronts the UI
                  environmentFile = cfg.envFile;
                  settings = {
                    # nixpkgs slskd module mkForces remote_configuration=false ("config immutable in
                    # store"). We DO want the runtime overlay: the broker keepalive re-PATCHes
                    # listen_port every 45s, so non-persistence across restart is fine. Override the
                    # module's mkForce — needs priority <50; our own mkForce would just collide at 50.
                    remote_configuration = lib.mkOverride 10 true;
                    web = {
                      inherit (cfg) port; # binds 0.0.0.0 → localhost for the broker client, LAN for Caddy
                      # login off — Authelia/LDAP gates the UI at Caddy (single SSO, no slskd login).
                      authentication.disabled = true;
                    };
                    soulseek = {
                      # 0.0.0.0 (not the veth): failover-robust — inbound DNAT lands on whichever veth
                      # is claimed, and the accepted socket's local addr = the DNAT'd veth IP so replies
                      # source correctly. Outbound is steered by the default route.
                      listen_ip_address = "0.0.0.0";
                      listen_port = cfg.listenPortSeed; # runtime API overlay (broker) overrides this
                      distributed_network.child_limit = cfg.childLimit;
                      connection = {
                        timeout = {
                          connect = cfg.connectTimeout;
                          inactivity = cfg.inactivityTimeout;
                          transfer = cfg.transferTimeout;
                        };
                        buffer = {
                          read = cfg.readBuffer;
                          write = cfg.writeBuffer;
                          transfer = cfg.transferBuffer;
                          write_queue = cfg.writeQueue;
                        };
                      };
                    };
                    shares = {
                      directories = cfg.shares;
                      cache = {
                        storage_mode = cfg.shareCacheMode;
                        workers = cfg.shareCacheWorkers;
                      };
                    };
                    directories = {
                      incomplete = cfg.incompleteDir;
                      downloads = cfg.downloadDir;
                    };
                    # slskd 0.26 (PR #1672): upload/download + groups live under a
                    # new top-level `transfers` key (the old `global` wrapper is gone).
                    transfers = {
                      upload = {
                        slots = cfg.uploadSlots;
                        speed_limit = cfg.uploadSpeedLimit;
                      };
                      download = {
                        slots = cfg.downloadSlots;
                        speed_limit = cfg.downloadSpeedLimit;
                      };
                      groups.leechers.upload.priority = cfg.leecherUploadPriority;
                    };
                    throttling.search.incoming = {
                      concurrency = cfg.searchConcurrency;
                      circuit_breaker = cfg.searchCircuitBreaker;
                      response_file_limit = cfg.searchResponseFileLimit;
                    };
                    # Prometheus /metrics on the web port; localhost Vector scrape (no auth).
                    metrics = {
                      enabled = true;
                      authentication.disabled = true;
                    };
                  };
                };

                systemd = {
                  # Downloads dir on NFS — created container-side (bind mount is present here at
                  # tmpfiles time, unlike the host where /mnt/media is an autofs not yet mounted).
                  # No chown: the share all_squashes, so a squashed root can't chown to cfg.uid.
                  tmpfiles.rules = [ "d ${cfg.downloadDir} 0770 - - - -" ];

                  services = {
                    # Route slskd's default via the claimed tunnel BEFORE it starts, so its very first
                    # outbound Soulseek connection is already VPN'd (fail-closed to blackhole otherwise).
                    slskd-vpn-prestart = {
                      description = "Route slskd egress through the VPN broker tunnel before start";
                      before = [ "slskd.service" ];
                      requiredBy = [ "slskd.service" ];
                      after = [ "network-online.target" ];
                      wants = [ "network-online.target" ];
                      path = [ pkgs.iproute2 ];
                      serviceConfig = {
                        Type = "oneshot";
                        RemainAfterExit = true;
                        ExecStart = "${brokerClient} prestart ${brokerCfg}";
                      };
                    };

                    # Renew the claim, reassert the default route, and PATCH slskd's live listenPort
                    # (via its runtime API — no reconnect). Self-heals on failover.
                    slskd-vpn-keepalive = {
                      description = "Renew VPN claim + pin slskd listenPort via the broker";
                      after = [ "slskd.service" ];
                      path = [ pkgs.iproute2 ];
                      serviceConfig = {
                        Type = "oneshot";
                        ExecStart = "${brokerClient} keepalive ${brokerCfg}";
                      };
                    };

                    slskd.serviceConfig = {
                      CPUQuota = "2000%";
                      CPUWeight = 100;
                      MemoryHigh = "10G";
                      MemoryMax = "14G";
                      # The module binds every share into ReadOnlyPaths; when the downloadDir is also
                      # shared, that RO bind collides with its own ReadWritePath and slskd refuses to
                      # start ("not writeable"). Rebuild ReadOnlyPaths (same alias-strip the module
                      # uses) minus the downloadDir, so a shared downloadDir stays read-write.
                      # cfg.shares are plain filesystem paths (no slskd alias syntax), so filter
                      # them directly — keep every share except the downloadDir, which must stay
                      # read-write for slskd to start.
                      ReadOnlyPaths = lib.mkForce (builtins.filter (p: p != cfg.downloadDir) cfg.shares);
                    };

                    # ── stopgap: periodic share rescan ──────────────────────────────────────
                    # shareCacheMode = memory + no built-in periodic scan → slskd only re-indexes
                    # on restart. Poke its own runtime API on loopback so newly imported/moved
                    # files in the shared dirs appear without a restart. slskd auth is disabled
                    # (Authelia gates the UI at Caddy) → no token needed.
                    slskd-rescan = {
                      description = "Trigger a slskd share rescan (pick up newly imported/moved files)";
                      after = [ "slskd.service" ];
                      serviceConfig = {
                        Type = "oneshot";
                        ExecStart = "${lib.getExe pkgs.curl} -fsS -m 30 -X PUT http://127.0.0.1:${toString cfg.port}/api/v0/shares";
                      };
                    };

                    # ── downloadDir housekeeping ────────────────────────────────────────────
                    # Two safe tiers only. Empty dirs are the shells Lidarr leaves after a
                    # hardlink import; failed_imports/ is what soularr moved aside and
                    # denylisted, so it is never retried and only grows. Non-empty top-level
                    # album dirs are deliberately untouched — they are stranded-but-importable
                    # (soularr only imports what it grabbed in the same run), and they are also
                    # live share content. soularr-reclaim drains those; this only sweeps after it.
                    soulseek-reap = {
                      description = "Reap empty album dirs + aged failed imports from the slskd download dir";
                      after = [ "slskd.service" ];
                      path = [
                        pkgs.findutils
                        pkgs.coreutils
                      ];
                      serviceConfig = {
                        Type = "oneshot";
                        ExecStart = pkgs.writeShellScript "soulseek-reap" ''
                          set -euo pipefail
                          find ${cfg.downloadDir} -maxdepth 1 -mindepth 1 -type d -empty \
                            -mmin +${toString cfg.reapEmptyMinutes} -delete
                          [ -d ${cfg.downloadDir}/failed_imports ] || exit 0
                          find ${cfg.downloadDir}/failed_imports -maxdepth 1 -mindepth 1 \
                            -mtime +${toString cfg.reapFailedImportsDays} -exec rm -rf {} +
                        '';
                        Nice = 19;
                        IOSchedulingClass = "idle";
                      };
                    };

                    # ── search history pruning ──────────────────────────────────────────────
                    # slskd has no search retention of its own and soularr searches
                    # continuously, so search.db only grows — and the UI's search view loads
                    # every record in one response, so it stalls long before the API does.
                    slskd-search-prune = {
                      description = "Delete slskd searches older than ${toString cfg.searchRetentionDays} days";
                      after = [ "slskd.service" ];
                      serviceConfig = {
                        Type = "oneshot";
                        ExecStart = "${pruneClient} --base http://127.0.0.1:${toString cfg.port} --retention-days ${toString cfg.searchRetentionDays}";
                        Nice = 19;
                        IOSchedulingClass = "idle";
                      };
                    };
                  };

                  timers = {
                    slskd-vpn-keepalive = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnBootSec = "40s";
                        OnUnitActiveSec = "45s";
                        Persistent = false;
                      };
                    };

                    slskd-rescan = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnBootSec = "15min";
                        OnUnitActiveSec = "60min";
                      };
                    };

                    # An hour behind soularr-reclaim (04:00), so the dirs it empties are
                    # already shells by the time the sweep runs. Cross-container ordering —
                    # no After= reaches across, the clock is the only handle.
                    soulseek-reap = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnCalendar = "05:00";
                        Persistent = true;
                      };
                    };

                    # 30min behind soulseek-reap so the two housekeeping jobs don't contend
                    # for slskd's SQLite and the NFS share at the same moment.
                    slskd-search-prune = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnCalendar = "05:30";
                        Persistent = true;
                      };
                    };
                  };
                };
              };
          };
      };
  };
}
