/*
  soularr — automated Soulseek acquisition of Lidarr's wanted list via slskd.
  https://github.com/mrusse/soularr

  Not in nixpkgs (bare script, 3 PyPI deps also absent) — binary is
  pkgs.homelab.soularr (see packages/soularr). Two units: a webui daemon
  (port cfg.port) and a oneshot `soularr`
  run fired on a timer (cfg.intervalSeconds). The whole config.ini is rendered
  here from typed options; the only secret (the Lidarr API key) is substituted at
  runtime from a bound sops secret, so nothing secret lands in the store.
*/
_: {
  den.aspects.containers._.soularr = {
    aspect-name = "containers.soularr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.soularr =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8265; # web UI (LAN); opened by _base
            defaultUid = 5035;
            defaultGid = 5035;
          })
          // {
            intervalSeconds = lib.mkOption {
              type = lib.types.int;
              default = 300;
              description = "How often the soularr run fires (Lidarr wanted list → slskd search/grab).";
            };
            stateDir = lib.mkOption {
              type = lib.types.str;
              default = "/var/lib/soularr";
              description = "Container state dir: config.ini, logs, lock file, failed-import denylist db.";
            };
            lidarrKeyFile = lib.mkOption {
              type = lib.types.str;
              default = "/run/secrets/soularr-lidarr-key";
              description = "In-container path to the raw Lidarr API key (bound sops secret); substituted into config.ini at runtime.";
            };

            # ── reclaim (soularr-reclaim) ──
            reclaimMinAgeMinutes = lib.mkOption {
              type = lib.types.int;
              default = 120;
              description = "Skip album folders modified more recently than this — a multi-file album may still be landing, since slskd writes each completed file into the download dir as it finishes.";
            };
            reclaimMaxFolders = lib.mkOption {
              type = lib.types.int;
              default = 0;
              description = "Cap on scan attempts per reclaim run; 0 = no cap. Each attempt is one Lidarr command, polled to completion.";
            };
            reclaimCommandTimeout = lib.mkOption {
              type = lib.types.int;
              default = 120;
              description = "Seconds to wait for a single Lidarr DownloadedAlbumsScan command to settle before giving up on that folder.";
            };

            # ── [Lidarr] ──
            lidarrUrl = lib.mkOption {
              type = lib.types.str;
              description = "Lidarr base URL (http://host:port).";
            };
            downloadDir = lib.mkOption {
              type = lib.types.str;
              default = "/mnt/media/complete/soulseek";
              description = "Completed-download dir, as both Lidarr and slskd see it (shared /mnt/media → hardlink import).";
            };
            disableSync = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Skip Lidarr library sync before searching.";
            };

            # ── [Slskd] ──
            slskdUrl = lib.mkOption {
              type = lib.types.str;
              description = "slskd base URL (http://host:port).";
            };
            slskdApiKey = lib.mkOption {
              type = lib.types.str;
              default = "disabled";
              description = "slskd API key. Non-secret: slskd auth is disabled so the key is ignored; slskd-api only needs a non-empty string.";
            };
            slskdUrlBase = lib.mkOption {
              type = lib.types.str;
              default = "/";
              description = "slskd url_base.";
            };
            deleteSearches = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Delete slskd searches after grabbing.";
            };
            stalledTimeout = lib.mkOption {
              type = lib.types.int;
              default = 3600;
              description = "Seconds before a stalled slskd download is abandoned.";
            };
            remoteQueueTimeout = lib.mkOption {
              type = lib.types.int;
              default = 300;
              description = "Seconds to wait while queued remotely on a peer.";
            };

            # ── [Release Settings] ──
            useSelectedLidarrRelease = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Match only Lidarr's selected release rather than any acceptable one.";
            };
            useMostCommonTracknum = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Resolve ambiguous track counts by the most common value.";
            };
            allowMultiDisc = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Accept multi-disc releases.";
            };
            acceptedCountries = lib.mkOption {
              type = lib.types.str;
              default = "Europe,Japan,United Kingdom,United States,[Worldwide],Australia,Canada";
              description = "Comma-separated accepted release countries.";
            };
            skipRegionCheck = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Skip the release-country check entirely.";
            };
            acceptedFormats = lib.mkOption {
              type = lib.types.str;
              default = "CD,Digital Media,Vinyl";
              description = "Comma-separated accepted release media formats.";
            };

            # ── [Search Settings] ──
            searchTimeout = lib.mkOption {
              type = lib.types.int;
              default = 5000;
              description = "Per-search timeout (ms).";
            };
            maximumPeerQueue = lib.mkOption {
              type = lib.types.int;
              default = 50;
              description = "Skip peers whose queue is longer than this.";
            };
            minimumPeerUploadSpeed = lib.mkOption {
              type = lib.types.int;
              default = 0;
              description = "Skip peers slower than this (bytes/s; 0 = no floor).";
            };
            minimumFilenameMatchRatio = lib.mkOption {
              type = lib.types.str;
              default = "0.8";
              description = "Minimum fuzzy filename-match ratio (0-1).";
            };
            minimumSearchInterval = lib.mkOption {
              type = lib.types.int;
              default = 5;
              description = "Minimum seconds between searches.";
            };
            allowedFiletypes = lib.mkOption {
              type = lib.types.str;
              default = "flac 24/192,flac 16/44.1,flac,mp3 320,mp3";
              description = "Preferred filetypes/qualities, most preferred first.";
            };
            albumPrependArtist = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Prepend the artist name to album searches.";
            };
            searchType = lib.mkOption {
              type = lib.types.str;
              default = "incrementing_page";
              description = "How soularr walks Lidarr's wanted list (incrementing_page | first_page | all).";
            };
            numberOfAlbumsToGrab = lib.mkOption {
              type = lib.types.int;
              default = 10;
              description = "Albums to attempt per run.";
            };
            searchSource = lib.mkOption {
              type = lib.types.str;
              default = "missing";
              description = "Lidarr source list to pull from (missing | cutoff_unmet | all).";
            };
            failedImportDenylist = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Denylist releases that fail import so they aren't retried forever.";
            };

            # ── [Download Settings] ──
            downloadFiltering = lib.mkOption {
              type = lib.types.bool;
              default = true;
              description = "Filter junk files out of grabbed folders.";
            };
            useExtensionWhitelist = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Keep only whitelisted non-audio extensions.";
            };
            extensionsWhitelist = lib.mkOption {
              type = lib.types.str;
              default = "lrc,nfo,txt";
              description = "Comma-separated non-audio extensions to keep when useExtensionWhitelist is set.";
            };
          };

        config.containers.soularr =
          let
            cfg = config.containerConfigs.soularr;
            # Resolved from host pkgs so the homelab overlay is in scope; the
            # store path pulls into the container closure via ExecStart.
            inherit (pkgs.homelab) soularr;
            soularrReclaim = lib.getExe pkgs.homelab.soularr-reclaim;
          in
          {
            inherit (cfg) autoStart;
            ephemeral = true;
            privateNetwork = true;
            inherit (cfg) hostBridge;
            localMacAddress = cfg.macAddress;
            inherit (cfg) localAddress bindMounts;

            config =
              { pkgs, lib, ... }:
              let
                # configparser wants Python-style True/False, not nix's true/false.
                b = x: if x then "True" else "False";
                configTemplate = pkgs.writeText "soularr-config.ini" (
                  lib.generators.toINI { } {
                    "Lidarr" = {
                      api_key = "@LIDARR_API_KEY@"; # substituted from the sops secret at runtime
                      host_url = cfg.lidarrUrl;
                      download_dir = cfg.downloadDir;
                      disable_sync = b cfg.disableSync;
                    };
                    "Slskd" = {
                      api_key = cfg.slskdApiKey;
                      host_url = cfg.slskdUrl;
                      url_base = cfg.slskdUrlBase;
                      download_dir = cfg.downloadDir;
                      delete_searches = b cfg.deleteSearches;
                      stalled_timeout = toString cfg.stalledTimeout;
                      remote_queue_timeout = toString cfg.remoteQueueTimeout;
                    };
                    "Release Settings" = {
                      use_selected_lidarr_release = b cfg.useSelectedLidarrRelease;
                      use_most_common_tracknum = b cfg.useMostCommonTracknum;
                      allow_multi_disc = b cfg.allowMultiDisc;
                      accepted_countries = cfg.acceptedCountries;
                      skip_region_check = b cfg.skipRegionCheck;
                      accepted_formats = cfg.acceptedFormats;
                    };
                    "Search Settings" = {
                      search_timeout = toString cfg.searchTimeout;
                      maximum_peer_queue = toString cfg.maximumPeerQueue;
                      minimum_peer_upload_speed = toString cfg.minimumPeerUploadSpeed;
                      minimum_filename_match_ratio = cfg.minimumFilenameMatchRatio;
                      minimum_search_interval = toString cfg.minimumSearchInterval;
                      allowed_filetypes = cfg.allowedFiletypes;
                      album_prepend_artist = b cfg.albumPrependArtist;
                      search_type = cfg.searchType;
                      number_of_albums_to_grab = toString cfg.numberOfAlbumsToGrab;
                      search_source = cfg.searchSource;
                      failed_import_denylist = b cfg.failedImportDenylist;
                    };
                    "Download Settings" = {
                      download_filtering = b cfg.downloadFiltering;
                      use_extension_whitelist = b cfg.useExtensionWhitelist;
                      extensions_whitelist = cfg.extensionsWhitelist;
                    };
                    "Logging" = {
                      level = "INFO";
                      format = "[%(levelname)s|%(module)s|L%(lineno)d] %(asctime)s: %(message)s";
                      datefmt = "%Y-%m-%dT%H:%M:%S%z";
                      log_to_file = "True";
                      log_file = "soularr.log";
                      max_bytes = "1048576";
                      backup_count = "3";
                    };
                  }
                );
              in
              {
                imports = [ (import ../_base.nix { inherit lib cfg; }) ]; # opens cfg.port (web) on LAN

                users.users.soularr = {
                  uid = lib.mkForce cfg.uid;
                  group = "soularr";
                  isSystemUser = true;
                };
                users.groups.soularr.gid = lib.mkForce cfg.gid;

                systemd = {
                  services = {
                    # Render config.ini into the state dir, injecting the Lidarr API key from
                    # the bound sops secret. Runs as container root (PID1) so it can read the
                    # 0400 secret; both app units wait on it.
                    soularr-config = {
                      description = "Render soularr config.ini (inject Lidarr API key)";
                      serviceConfig = {
                        Type = "oneshot";
                        RemainAfterExit = true;
                        ExecStart = pkgs.writeShellScript "soularr-config" ''
                          set -euo pipefail
                          key=$(cat ${cfg.lidarrKeyFile})
                          ${pkgs.gnused}/bin/sed "s|@LIDARR_API_KEY@|$key|" ${configTemplate} > ${cfg.stateDir}/config.ini
                          chown ${toString cfg.uid}:${toString cfg.gid} ${cfg.stateDir}/config.ini
                          chmod 600 ${cfg.stateDir}/config.ini
                        '';
                      };
                    };

                    soularr-webui = {
                      description = "soularr web UI";
                      after = [
                        "network-online.target"
                        "soularr-config.service"
                      ];
                      wants = [ "network-online.target" ];
                      requires = [ "soularr-config.service" ];
                      wantedBy = [ "multi-user.target" ];
                      serviceConfig = {
                        Type = "simple";
                        User = "soularr";
                        Group = "soularr";
                        WorkingDirectory = cfg.stateDir;
                        ExecStart = "${soularr}/bin/soularr-webui --var-dir ${cfg.stateDir} --host 0.0.0.0 --port ${toString cfg.port}";
                        Restart = "on-failure";
                        RestartSec = "10s";
                      };
                    };

                    # One sync pass (Lidarr wanted → slskd search/grab → trigger import),
                    # fired by the timer. systemd already serializes the oneshot (a new timer
                    # tick won't start while the previous run is still active), so the only job
                    # of soularr's own .soularr.lock is to strand us: it's a static "locked"
                    # marker with no PID/liveness check, so a run killed mid-flight (redeploy,
                    # reboot, OOM) orphans it forever and every subsequent run then exits 1
                    # silently before it even opens its logger. Clear any stale lock up front.
                    soularr-run = {
                      description = "soularr: sync Lidarr wanted list to slskd";
                      after = [
                        "network-online.target"
                        "soularr-config.service"
                      ];
                      wants = [ "network-online.target" ];
                      requires = [ "soularr-config.service" ];
                      serviceConfig = {
                        Type = "oneshot";
                        User = "soularr";
                        Group = "soularr";
                        WorkingDirectory = cfg.stateDir;
                        ExecStartPre = "${pkgs.coreutils}/bin/rm -f ${cfg.stateDir}/.soularr.lock";
                        ExecStart = "${soularr}/bin/soularr --config-dir ${cfg.stateDir} --var-dir ${cfg.stateDir}";
                        # ── guardrail ── batch acquisition run; keep it off the host's neck.
                        CPUQuota = "400%";
                        CPUWeight = 20;
                        Nice = 19;
                        IOSchedulingClass = "idle";
                      };
                    };

                    # ── reclaim ── a soularr run only imports what it grabbed in that same run,
                    # so anything stranded by a crash/stall (or by an earlier search_type=all
                    # sweep) sits in the download dir forever: real music nothing will ever look
                    # at again. Replay the import half per album folder — DownloadedAlbumsScan
                    # takes ONE album folder, never the parent (the parent has no audio directly
                    # inside it and comes back "Failed to import" in ~13ms). Runs as container
                    # root to read the 0400 key, same as soularr-config. ExecCondition keeps it
                    # off the dir while a soularr sweep is mid-import there.
                    soularr-reclaim = {
                      description = "Reclaim stranded soulseek downloads via per-album Lidarr scans";
                      after = [ "network-online.target" ];
                      wants = [ "network-online.target" ];
                      environment = {
                        LIDARR_URL = cfg.lidarrUrl;
                        LIDARR_KEY_FILE = cfg.lidarrKeyFile;
                        DOWNLOAD_DIR = cfg.downloadDir;
                        RECLAIM_MIN_AGE_MINUTES = toString cfg.reclaimMinAgeMinutes;
                        RECLAIM_MAX_FOLDERS = toString cfg.reclaimMaxFolders;
                        RECLAIM_COMMAND_TIMEOUT = toString cfg.reclaimCommandTimeout;
                      };
                      serviceConfig = {
                        Type = "oneshot";
                        ExecCondition = "${lib.getExe pkgs.bash} -c '! systemctl is-active --quiet soularr-run.service'";
                        ExecStart = soularrReclaim;
                        Nice = 19;
                        IOSchedulingClass = "idle";
                      };
                    };
                  };

                  timers = {
                    # An hour ahead of slskd's soulseek-reap (05:00), which sweeps the empty
                    # dirs this leaves behind. Cross-container ordering — the clock is the
                    # only handle.
                    soularr-reclaim = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnCalendar = "04:00";
                        Persistent = true;
                      };
                    };

                    soularr-run = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnBootSec = "2min";
                        OnUnitActiveSec = "${toString cfg.intervalSeconds}s";
                        Persistent = false;
                      };
                    };
                  };
                };
              };
          };
      };
  };
}
