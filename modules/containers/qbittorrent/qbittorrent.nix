/*
  qbittorrent — containerized qBittorrent (headless) BitTorrent client
  https://www.qbittorrent.org
*/
_: {
  den.aspects.containers._.qbittorrent = {
    aspect-name = "containers.qbittorrent";
    includes = [ ];

    provides.to-hosts.nixos =
      { config, lib, ... }:
      let
        # ── [BitTorrent] section — maps to Session\* conf keys ──
        bitTorrentSubmodule = lib.types.submodule {
          options = {
            defaultSavePath = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/download";
              description = "Session\\DefaultSavePath — default save location";
            };
            tempPath = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/download";
              description = "Session\\TempPath — incomplete-download location";
            };
            tempPathEnabled = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Session\\TempPathEnabled — keep incomplete torrents in tempPath";
            };
            finishedTorrentExportDirectory = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/torrents";
              description = "Session\\FinishedTorrentExportDirectory — flat .torrent backup dir";
            };
            disableAutoTMMByDefault = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\DisableAutoTMMByDefault — false = AutoTMM on (category drives path)";
            };
            port = lib.mkOption {
              type = lib.types.port;
              default = 6881;
              description = "Session\\Port — listen port seed (qbt-net-config pins from sops)";
            };

            # ── disk / performance ──
            diskIOType = lib.mkOption {
              type = lib.types.enum [
                "Default"
                "MMap"
                "Posix"
                "SimplePreadPwrite"
              ];
              default = "Default";
              description = "Session\\DiskIOType";
            };
            asyncIOThreadsCount = lib.mkOption {
              type = lib.types.int;
              default = 10;
              description = "Session\\AsyncIOThreadsCount — disk I/O worker threads";
            };
            hashingThreadsCount = lib.mkOption {
              type = lib.types.int;
              default = 1;
              description = "Session\\HashingThreadsCount — parallel piece-hash threads";
            };
            filePoolSize = lib.mkOption {
              type = lib.types.int;
              default = 100;
              description = "Session\\FilePoolSize — max concurrently-open files (raise LimitNOFILE to match)";
            };
            checkingMemUsageSize = lib.mkOption {
              type = lib.types.int;
              default = 32;
              description = "Session\\CheckingMemUsageSize — hash-check buffer (MiB)";
            };
            diskQueueSize = lib.mkOption {
              type = lib.types.int;
              default = 1048576;
              description = "Session\\DiskQueueSize — write queue (bytes)";
            };
            coalesceReadWrite = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\CoalesceReadWrite — batch disk I/O";
            };
            pieceExtentAffinity = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\PieceExtentAffinity — bias the piece picker toward pieces in the same file extent, so concurrent peers pull from a narrow region of the file instead of spreading seeks across the whole torrent";
            };
            suggestMode = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\SuggestMode — send SUGGEST messages naming pieces already in cache so peers request those first. libtorrent maps this to suggest_read_cache, which needs a disk backend that tracks cached pieces";
            };
            chokingAlgorithm = lib.mkOption {
              type = lib.types.enum [
                "FixedSlots"
                "RateBased"
              ];
              default = "FixedSlots";
              description = "Session\\ChokingAlgorithm — RateBased auto-scales upload slots to rate";
            };
            seedChokingAlgorithm = lib.mkOption {
              type = lib.types.enum [
                "RoundRobin"
                "FastestUpload"
                "AntiLeech"
              ];
              default = "RoundRobin";
              description = "Session\\SeedChokingAlgorithm — how seeding torrents pick which peers fill the upload slots. FastestUpload concentrates slots on the fastest peers (with a finite maxUploads) so the slow tail is choked — fewer concurrent random reads off the NFS-backed library";
            };
            queueingSystemEnabled = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Session\\QueueingSystemEnabled — false = seed all torrents (no active cap)";
            };

            # ── connections / bandwidth ──
            maxConnections = lib.mkOption {
              type = lib.types.int;
              default = 500;
              description = "Session\\MaxConnections — global connection limit (-1 unlimited)";
            };
            maxConnectionsPerTorrent = lib.mkOption {
              type = lib.types.int;
              default = 100;
              description = "Session\\MaxConnectionsPerTorrent";
            };
            maxUploads = lib.mkOption {
              type = lib.types.int;
              default = 20;
              description = "Session\\MaxUploads — global upload slots (-1 unlimited)";
            };
            maxUploadsPerTorrent = lib.mkOption {
              type = lib.types.int;
              default = 4;
              description = "Session\\MaxUploadsPerTorrent (-1 unlimited)";
            };
            socketSendBufferSize = lib.mkOption {
              type = lib.types.int;
              default = 0;
              description = "Session\\SocketSendBufferSize — OS socket send buffer bytes (0 = auto)";
            };
            socketReceiveBufferSize = lib.mkOption {
              type = lib.types.int;
              default = 0;
              description = "Session\\SocketReceiveBufferSize — OS socket recv buffer bytes (0 = auto)";
            };
            sendBufferWatermark = lib.mkOption {
              type = lib.types.int;
              default = 500;
              description = "Session\\SendBufferWatermark — per-peer send buffer high watermark (KiB)";
            };
            sendBufferLowWatermark = lib.mkOption {
              type = lib.types.int;
              default = 10;
              description = "Session\\SendBufferLowWatermark (KiB)";
            };
            sendBufferWatermarkFactor = lib.mkOption {
              type = lib.types.int;
              default = 50;
              description = "Session\\SendBufferWatermarkFactor — % of peer rate";
            };
            connectionSpeed = lib.mkOption {
              type = lib.types.int;
              default = 30;
              description = "Session\\ConnectionSpeed — outbound connection attempts/sec";
            };
            socketBacklogSize = lib.mkOption {
              type = lib.types.int;
              default = 30;
              description = "Session\\SocketBacklogSize — inbound listen backlog";
            };
            multiConnectionsPerIp = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\MultiConnectionsPerIp — allow more than one peer connection from a single IP (peers behind shared NAT, or several clients on one seedbox)";
            };

            # ── peer turnover — libtorrent's periodic peer churn ──
            # Fires when total connections reach maxConnections * cutoff%, or (failing
            # that) when a single torrent reaches maxConnectionsPerTorrent * cutoff%.
            # Which peers get dropped is NOT tunable: compare_disconnect_peer ranks on
            # is_interesting / is_seed / payload DOWNLOAD rate, all of which are
            # degenerate or inverted on a completed torrent, and it never looks at
            # upload rate. These three only control how often and how many.
            peerTurnover = lib.mkOption {
              type = lib.types.int;
              default = 4;
              description = "Session\\PeerTurnover — % of a torrent's peers to disconnect per pass. Note 0 does NOT disable it: libtorrent floors the count at 1 peer (max(n * pct / 100, 1)), capped by the number of connect candidates";
            };
            peerTurnoverCutOff = lib.mkOption {
              type = lib.types.int;
              default = 90;
              description = "Session\\PeerTurnoverCutOff — only churn once connections reach this % of the limit; 100 = only at the limit itself";
            };
            peerTurnoverInterval = lib.mkOption {
              type = lib.types.int;
              default = 300;
              description = "Session\\PeerTurnoverInterval — seconds between churn passes";
            };

            # ── protocol ──
            dht = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Session\\DHTEnabled";
            };
            pex = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Session\\PeXEnabled";
            };
            lsd = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Session\\LSDEnabled";
            };
            encryption = lib.mkOption {
              type = lib.types.enum [
                0
                1
                2
              ];
              default = 0;
              description = "Session\\Encryption — 0=prefer 1=require 2=off";
            };
            preallocation = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\Preallocation — false for NFS";
            };
            torrentContentLayout = lib.mkOption {
              type = lib.types.str;
              default = "Original";
              description = "Session\\TorrentContentLayout";
            };
            btProtocol = lib.mkOption {
              type = lib.types.enum [
                "Both"
                "TCP"
                "uTP"
              ];
              default = "Both";
              description = "Session\\BTProtocol — Both|TCP|uTP. TCP avoids uTP/LEDBAT latency-collapse over a jittery VPN tunnel";
            };
            announceToAllTrackers = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Session\\AnnounceToAllTrackers — announce to every tracker in a tier simultaneously";
            };
          };
        };

        # ── [Preferences] section — maps to WebUI\* conf keys ──
        preferencesSubmodule = lib.types.submodule {
          options = {
            webUIAddress = lib.mkOption {
              type = lib.types.str;
              default = "*";
              description = "WebUI\\Address";
            };
            webUILocalHostAuth = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "WebUI\\LocalHostAuth — false = localhost bypasses auth";
            };
            authSubnetWhitelistEnabled = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "WebUI\\AuthSubnetWhitelistEnabled";
            };
            authSubnetWhitelist = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = "WebUI\\AuthSubnetWhitelist — subnets that bypass auth";
            };
          };
        };
      in
      {
        options.containerConfigs.qbittorrent =
          (import ../_options.nix {
            inherit lib;
            defaultUid = 5019;
            defaultGid = 5019;
          })
          // {
            webuiPort = lib.mkOption {
              type = lib.types.port;
              default = 8080;
              description = "WebUI/API port (WebUI\\Port + service arg)";
            };
            profileDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/qbittorrent";
              description = "qBittorrent --profile dir";
            };
            broker = lib.mkOption {
              type = lib.types.str;
              description = "Base URL of the vpn-gw tunnel broker (http://<vpn-gw-ip>:<control-port>)";
            };
            mam = lib.mkOption {
              default = { };
              description = ''
                MyAnonaMouse dynamic-seedbox re-lock. On VPN failover the exit IP changes; the
                broker keepalive POSTs the new IP to MAM (from the claimed tunnel) so the IP/ASN-
                locked session stays valid. Requires an ASN-locked MAM session whose allowlist
                covers every pool exit ASN. Off by default.
              '';
              type = lib.types.submodule {
                options = {
                  enable = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Enable the MAM dynamic-seedbox updater in the keepalive tick";
                  };
                  endpoint = lib.mkOption {
                    type = lib.types.str;
                    default = "https://t.myanonamouse.net/json/dynamicSeedbox.php";
                    description = "MAM dynamic-seedbox API endpoint";
                  };
                  seedSecretPath = lib.mkOption {
                    type = lib.types.str;
                    default = "/run/secrets/mam-vpn-id";
                    description = "In-container path to the mam_id seed cookie (sops, bind-mounted)";
                  };
                  floorSeconds = lib.mkOption {
                    type = lib.types.int;
                    default = 3600;
                    description = "Min seconds between change attempts (MAM rate-limits changes to 1/hr)";
                  };
                  refreshSeconds = lib.mkOption {
                    type = lib.types.int;
                    default = 3600;
                    description = "Keepalive cadence: re-POST the unchanged IP this often so MAM rotates the cookie and the session never expires";
                  };
                };
              };
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
                      description = "This client's address on the veth (must match the broker's DNAT target)";
                    };
                    gateway = lib.mkOption {
                      type = lib.types.str;
                      description = "vpn-gw's address on the veth (egress next-hop)";
                    };
                    table = lib.mkOption {
                      type = lib.types.int;
                      description = "Routing-table id for this tunnel's egress";
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
            watchDir = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/watch";
              description = "Watch root (per-category subdirs) for the importer";
            };
            categories = lib.mkOption {
              type = lib.types.attrsOf lib.types.str;
              default = { };
              description = "Category name -> save path";
            };
            watchIntervalSec = lib.mkOption {
              type = lib.types.int;
              default = 10;
              description = "watch-dir poll cadence";
            };
            tagSyncIntervalSec = lib.mkOption {
              type = lib.types.int;
              default = 60;
              description = "category->tag mirror cadence";
            };
            reannounceIntervalSec = lib.mkOption {
              type = lib.types.int;
              default = 60;
              description = "tracker re-announce cadence for stalled zero-progress torrents";
            };
            qbittorrentExporter = {
              enable = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Run martabal qbittorrent-exporter";
              };
              port = lib.mkOption {
                type = lib.types.port;
                default = 8090;
                description = "Exporter metrics port";
              };
              highCardinality = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Per-torrent info labels for the Torrents table";
              };
            };
            bitTorrentConfig = lib.mkOption {
              type = bitTorrentSubmodule;
              default = { };
              description = "[BitTorrent] session settings";
            };
            preferences = lib.mkOption {
              type = preferencesSubmodule;
              default = { };
              description = "[Preferences] settings";
            };
            onTorrentFinished = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                Program run on torrent completion ([AutoRun] enabled/program — the
                bare keys are qBittorrent's legacy "on finished" trigger). qBittorrent
                substitutes %I (infohash v1), %N (name), %F (content path), etc.
              '';
            };
            onTorrentAdded = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                Program run when a torrent is added ([AutoRun] OnTorrentAdded\Program).
                Same %-substitutions as onTorrentFinished.
              '';
            };
            qbitManage = {
              enable = lib.mkOption {
                type = lib.types.bool;
                default = false;
                description = "Run periodic qbit-manage passes driven by qbitManage.settings";
              };
              scheduleMinutes = lib.mkOption {
                type = lib.types.int;
                default = 1440;
                description = "Minutes between qbit-manage runs";
              };
              settings = lib.mkOption {
                type = lib.types.attrs;
                default = { };
                description = ''
                  Full qbit-manage config.yml content (commands, qbt, directory,
                  tracker, nohardlinks, …), rendered verbatim. The host override owns
                  policy — e.g. keep every rem_*/share_limits command false for tag-only.
                '';
              };
            };
          };

        config = {
          containers.qbittorrent =
            let
              cfg = config.containerConfigs.qbittorrent;
              bt = cfg.bitTorrentConfig;
              pref = cfg.preferences;
              # VPN tunnel pool — the client's OWN wiring (decoupled from vpn-gw). Each entry
              # is a p2p veth into br-vpnN with the routing bits for egress. The broker (on
              # vpn-gw) selects which tunnel to bind and hands its name back; the broker client
              # maps that name -> veth via this pool.
              inherit (cfg) vpnPool;
              # Killswitch-safe seed for the session's initial bind (first pool entry). Binding
              # to a veth is inherently killswitched — a down tunnel drops traffic, never leaks
              # to eth0. The broker client rebinds to the live claimed tunnel at prestart and on
              # every keepalive tick.
              seedTunnel = builtins.elemAt vpnPool 0;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              timeoutStartSec = "5min";
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              inherit (cfg) bindMounts;

              # main eth0 = download VLAN (LAN: webui/arr/NFS + default route); one p2p
              # veth per tunnel into br-vpnN. Only the torrent session binds to a tunnel.
              extraVeths = lib.listToAttrs (
                map (
                  v:
                  lib.nameValuePair v.veth {
                    hostBridge = v.bridge;
                    localAddress = "${v.ip}/24";
                  }
                ) vpnPool
              );

              config =
                { pkgs, lib, ... }:
                let
                  # Rendered read-only into the store, then copied to a writable
                  # config-dir at preStart so qbit-manage can write its logs beside it.
                  # Content (commands/qbt/tracker/nohardlinks) comes entirely from the
                  # host override via cfg.qbitManage.settings.
                  qbmConfig = (pkgs.formats.yaml { }).generate "qbit-manage.yml" cfg.qbitManage.settings;
                  # Broker client config: broker URL + local WebUI + conf path + tunnel->veth map.
                  brokerClientCfg = pkgs.writeText "qbt-vpn-broker.json" (
                    builtins.toJSON {
                      inherit (cfg) broker;
                      api = "http://127.0.0.1:${toString cfg.webuiPort}";
                      conf = "${cfg.profileDir}/qBittorrent/config/qBittorrent.conf";
                      client_name = "qbittorrent";
                      pool = lib.listToAttrs (map (v: lib.nameValuePair v.tunnel { inherit (v) veth ip; }) vpnPool);
                      # Persistent dir (profileDir is bind-mounted → survives the ephemeral
                      # container) for cross-restart state: last-held tunnel (tunnel-preference)
                      # + MAM cookie jar / last-set-IP state.
                      state_dir = "${cfg.profileDir}/.vpnbroker";
                      mam = {
                        inherit (cfg.mam) enable endpoint;
                        seed_path = cfg.mam.seedSecretPath;
                        floor_seconds = cfg.mam.floorSeconds;
                        refresh_seconds = cfg.mam.refreshSeconds;
                      };
                    }
                  );
                in
                {
                  imports = [
                    (import ../_base.nix {
                      inherit lib cfg;
                      hasPort = false;
                    })
                  ];

                  # Static forwarded peer port + isolation behind vpn-gw/VPN → no in-container firewall.
                  networking.firewall.enable = lib.mkForce false;

                  services.qbittorrent = {
                    enable = true;
                    inherit (cfg) profileDir;
                    inherit (cfg) webuiPort;
                    user = "qbittorrent";
                    group = "qbittorrent";
                    serverConfig = {
                      BitTorrent = {
                        "Session\\DefaultSavePath" = bt.defaultSavePath;
                        "Session\\TempPathEnabled" = bt.tempPathEnabled;
                        "Session\\TempPath" = bt.tempPath;
                        "Session\\FinishedTorrentExportDirectory" = bt.finishedTorrentExportDirectory;
                        "Session\\DisableAutoTMMByDefault" = bt.disableAutoTMMByDefault;
                        "Session\\DiskIOType" = bt.diskIOType;
                        "Session\\AsyncIOThreadsCount" = bt.asyncIOThreadsCount;
                        "Session\\HashingThreadsCount" = bt.hashingThreadsCount;
                        "Session\\FilePoolSize" = bt.filePoolSize;
                        "Session\\CheckingMemUsageSize" = bt.checkingMemUsageSize;
                        "Session\\DiskQueueSize" = bt.diskQueueSize;
                        "Session\\CoalesceReadWrite" = bt.coalesceReadWrite;
                        "Session\\PieceExtentAffinity" = bt.pieceExtentAffinity;
                        "Session\\SuggestMode" = bt.suggestMode;
                        "Session\\ChokingAlgorithm" = bt.chokingAlgorithm;
                        "Session\\SeedChokingAlgorithm" = bt.seedChokingAlgorithm;
                        "Session\\QueueingSystemEnabled" = bt.queueingSystemEnabled;
                        "Session\\BTProtocol" = bt.btProtocol;
                        "Session\\AnnounceToAllTrackers" = bt.announceToAllTrackers;
                        # bind the libtorrent session to a tunnel interface — torrent
                        # traffic egresses the VPN, everything else stays on eth0. Inherent
                        # killswitch: if the interface is down, no torrent leaks. Seed = wg1;
                        # vpn_select.py rebinds to a live tunnel at runtime via the API.
                        "Session\\Interface" = seedTunnel.veth;
                        "Session\\InterfaceName" = seedTunnel.veth;
                        "Session\\MaxConnections" = bt.maxConnections;
                        "Session\\MaxConnectionsPerTorrent" = bt.maxConnectionsPerTorrent;
                        "Session\\MaxUploads" = bt.maxUploads;
                        "Session\\MaxUploadsPerTorrent" = bt.maxUploadsPerTorrent;
                        "Session\\SocketSendBufferSize" = bt.socketSendBufferSize;
                        "Session\\SocketReceiveBufferSize" = bt.socketReceiveBufferSize;
                        "Session\\SendBufferWatermark" = bt.sendBufferWatermark;
                        "Session\\SendBufferLowWatermark" = bt.sendBufferLowWatermark;
                        "Session\\SendBufferWatermarkFactor" = bt.sendBufferWatermarkFactor;
                        "Session\\ConnectionSpeed" = bt.connectionSpeed;
                        "Session\\SocketBacklogSize" = bt.socketBacklogSize;
                        "Session\\MultiConnectionsPerIp" = bt.multiConnectionsPerIp;
                        "Session\\PeerTurnover" = bt.peerTurnover;
                        "Session\\PeerTurnoverCutOff" = bt.peerTurnoverCutOff;
                        "Session\\PeerTurnoverInterval" = bt.peerTurnoverInterval;
                        "Session\\DHTEnabled" = bt.dht;
                        "Session\\PeXEnabled" = bt.pex;
                        "Session\\LSDEnabled" = bt.lsd;
                        "Session\\Encryption" = bt.encryption;
                        "Session\\Preallocation" = bt.preallocation;
                        "Session\\TorrentContentLayout" = bt.torrentContentLayout;
                      };
                      Preferences = {
                        "WebUI\\Port" = cfg.webuiPort;
                        "WebUI\\Address" = pref.webUIAddress;
                        "WebUI\\LocalHostAuth" = pref.webUILocalHostAuth;
                        "WebUI\\AuthSubnetWhitelistEnabled" = pref.authSubnetWhitelistEnabled;
                        "WebUI\\AuthSubnetWhitelist" = lib.concatStringsSep "," pref.authSubnetWhitelist;
                      };
                    }
                    // lib.optionalAttrs (cfg.onTorrentFinished != null || cfg.onTorrentAdded != null) {
                      # [AutoRun]: bare enabled/program = on-finished (legacy keys);
                      # OnTorrentAdded\* = on-added. Either or both may be set.
                      AutoRun =
                        (lib.optionalAttrs (cfg.onTorrentFinished != null) {
                          enabled = true;
                          program = cfg.onTorrentFinished;
                        })
                        // (lib.optionalAttrs (cfg.onTorrentAdded != null) {
                          "OnTorrentAdded\\Enabled" = true;
                          "OnTorrentAdded\\Program" = cfg.onTorrentAdded;
                        });
                    };
                  };

                  users.users.qbittorrent = {
                    uid = lib.mkForce cfg.uid;
                    group = "qbittorrent";
                    isSystemUser = true;
                  };
                  users.groups.qbittorrent.gid = lib.mkForce cfg.gid;

                  environment.systemPackages = [ pkgs.qbittorrent-nox ];

                  systemd = {
                    services = {
                      # Per-tunnel egress routing. qbt's default stays on eth0 (LAN); only
                      # traffic bound to a vpnN interface (the torrent session) routes out
                      # its tunnel (oif rule → table → default via vpn-gw). The NAT-PMP
                      # gateway is routed via the SELECTED tunnel so natpmpc (which can't
                      # bind an interface) reaches the provider through it.
                      qbt-routing = {
                        description = "Per-tunnel VPN egress routing";
                        after = [ "network-online.target" ];
                        wants = [ "network-online.target" ];
                        before = [ "qbittorrent.service" ];
                        wantedBy = [ "multi-user.target" ];
                        path = [ pkgs.iproute2 ];
                        serviceConfig = {
                          Type = "oneshot";
                          RemainAfterExit = true;
                        };
                        script = ''
                          set -eu
                          while ip rule del priority 200 2>/dev/null; do :; done
                          ${lib.concatMapStringsSep "\n" (v: ''
                            ip link set dev ${v.veth} mtu 1420
                            ip route replace default via ${v.gateway} dev ${v.veth} table ${toString v.table}
                            ip rule add oif ${v.veth} lookup ${toString v.table} priority 200
                          '') vpnPool}
                        '';
                      };

                      # nspawn: user namespace remap breaks bind-mount ownership;
                      # raise LimitNOFILE for FilePoolSize.
                      # ExecStartPre (mkAfter, run as root via +): after the module's `install`
                      # step copies a fresh conf, the broker client claims a tunnel and patches
                      # Session\Port/Interface/AnnounceIP so qbt binds the live tunnel on its very
                      # first announce. Fail-closed if the broker isn't ready (seed binding kept).
                      # Boot-race self-heal: qbt can self-terminate (clean exit 0) when the
                      # network/vpn veth/broker isn't settled yet at boot; with the module
                      # default Restart=no it then stays dead forever. Restart=always retries
                      # until the environment settles (on-failure would NOT fire — the race
                      # exit is 0). StartLimit turns a genuinely-broken config into a visible
                      # failed unit instead of an infinite loop.
                      qbittorrent = {
                        startLimitIntervalSec = 300;
                        startLimitBurst = 10;
                        serviceConfig = {
                          PrivateUsers = lib.mkForce false;
                          ReadWritePaths = [
                            "/mnt/media"
                            cfg.profileDir
                          ];
                          LimitNOFILE = 1048576;
                          Restart = "always";
                          RestartSec = "10s";
                          ExecStartPre = lib.mkAfter [
                            "+${lib.getExe pkgs.python3} ${./vpn_broker_client.py} prestart ${brokerClientCfg}"
                          ];
                        };
                      };

                      # ── Keepalive: renew the broker claim (re-lease + refresh TTL) and pin
                      #    the current listen_port + announce_ip via the WebUI API. Reconciles
                      #    qbt's bound interface with the claimed tunnel; self-heals on failover
                      #    (a voided tunnel → next tick claims a healthy one). Selection lives in
                      #    the broker now — no probing here. ──
                      qbt-vpn-keepalive = {
                        description = "Renew VPN claim + pin listen_port/announce_ip via the broker";
                        after = [ "qbittorrent.service" ];
                        # curl is only needed for the MAM dynamic-seedbox POST (--interface <veth>).
                        path = lib.optionals cfg.mam.enable [ pkgs.curl ];
                        serviceConfig = {
                          Type = "oneshot";
                          ExecStart = "${lib.getExe pkgs.python3} ${./vpn_broker_client.py} keepalive ${brokerClientCfg}";
                        };
                      };

                      # ── watch/<category>/ → add with category, move .torrent out ──
                      qbt-watch = {
                        description = "qBittorrent watch-dir importer";
                        after = [ "qbittorrent.service" ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "qbittorrent";
                          ExecStart =
                            "${pkgs.python3}/bin/python3 ${./watch.py}"
                            + " --api http://127.0.0.1:${toString cfg.webuiPort}"
                            + " --watch-dir ${cfg.watchDir}"
                            + " --backup-dir ${bt.finishedTorrentExportDirectory}"
                            + " --categories ${lib.concatStringsSep "," (lib.attrNames cfg.categories)}";
                        };
                      };

                      # ── re-announce zero-progress torrents whose trackers have all errored ──
                      qbt-reannounce = {
                        description = "qBittorrent tracker re-announce for stalled zero-progress torrents";
                        after = [ "qbittorrent.service" ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "qbittorrent";
                          ExecStart = "${pkgs.python3}/bin/python3 ${./reannounce.py} --api http://127.0.0.1:${toString cfg.webuiPort}";
                        };
                      };

                      # ── mirror category -> tag (Flood shows tags, not categories) ──
                      qbt-tag-sync = {
                        description = "qBittorrent category->tag mirror";
                        after = [ "qbittorrent.service" ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "qbittorrent";
                          ExecStart = "${pkgs.python3}/bin/python3 ${./tag_sync.py} --api http://127.0.0.1:${toString cfg.webuiPort}";
                        };
                      };

                      # ── qbit-manage maintenance pass (tag-only per host settings) ──
                      # Config is read-only in the store; copy it into a writable config-dir
                      # in the persistent profile so qbit-manage can write logs/ beside it.
                      qbit-manage = lib.mkIf cfg.qbitManage.enable {
                        description = "qbit-manage maintenance pass";
                        after = [ "qbittorrent.service" ];
                        preStart = ''
                          ${pkgs.coreutils}/bin/install -d -m 0700 ${cfg.profileDir}/.qbit_manage/logs
                          ${pkgs.coreutils}/bin/install -m 0600 ${qbmConfig} ${cfg.profileDir}/.qbit_manage/config.yml
                        '';
                        serviceConfig = {
                          Type = "oneshot";
                          User = "qbittorrent";
                          Group = "qbittorrent";
                          ExecStart = "${lib.getExe' pkgs.qbit-manage "qbit-manage"} --config-dir ${cfg.profileDir}/.qbit_manage --run";
                        };
                      };

                      # ── Prometheus exporter (martabal qbittorrent-exporter) ──
                      qbt-exporter = lib.mkIf cfg.qbittorrentExporter.enable {
                        description = "qBittorrent Prometheus exporter";
                        after = [ "qbittorrent.service" ];
                        wantedBy = [ "multi-user.target" ];
                        environment = {
                          EXPORTER_HOST = "127.0.0.1";
                          EXPORTER_PORT = toString cfg.qbittorrentExporter.port;
                          QBITTORRENT_BASE_URL = "http://127.0.0.1:${toString cfg.webuiPort}";
                          ENABLE_HIGH_CARDINALITY = lib.boolToString cfg.qbittorrentExporter.highCardinality;
                          ENABLE_LABEL_WITH_TAGS = "true";
                        };
                        serviceConfig = {
                          ExecStart = lib.getExe pkgs.prometheus-qbittorrent-exporter;
                          User = "qbittorrent";
                          Restart = "on-failure";
                          RestartSec = "10s";
                        };
                      };
                    };

                    timers = {
                      qbt-vpn-keepalive = {
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnBootSec = "40s";
                          # Renew faster than the broker claim TTL (120s) AND the natpmp lease
                          # (60s) so neither the claim nor the port forward lapses between runs.
                          OnUnitActiveSec = "45s";
                          Persistent = false;
                        };
                      };
                      qbt-watch = {
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnBootSec = "30s";
                          OnUnitActiveSec = "${toString cfg.watchIntervalSec}s";
                          Persistent = false;
                        };
                      };
                      qbt-reannounce = {
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnBootSec = "45s";
                          OnUnitActiveSec = "${toString cfg.reannounceIntervalSec}s";
                          Persistent = false;
                        };
                      };
                      qbt-tag-sync = {
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnBootSec = "35s";
                          OnUnitActiveSec = "${toString cfg.tagSyncIntervalSec}s";
                          Persistent = false;
                        };
                      };
                      qbit-manage = lib.mkIf cfg.qbitManage.enable {
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnBootSec = "5m";
                          OnUnitActiveSec = "${toString cfg.qbitManage.scheduleMinutes}m";
                          Persistent = false;
                        };
                      };
                    };
                  };
                };
            };
        };
      };
  };
}
