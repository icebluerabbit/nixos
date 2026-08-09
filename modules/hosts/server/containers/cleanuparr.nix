/*
  cleanuparr — host configuration override for cleanuparr container

  ROLLOUT: this is deliberately staged. Cleanuparr is the only thing in this
  homelab allowed to remove a torrent, and every tracker here is private, so a
  misconfigured rule costs ratio and H&R points, not just disk. Each phase runs
  long enough to be believed before the next one is enabled.

  ── Phase 0 — observe only (CURRENT) ────────────────────────────────────────
  Tagging runs; nothing strikes, nothing is removed from a queue, nothing is
  deleted. `dry_run` stays false on purpose: dry run also suppresses the
  unlinked/dead tagging, which is the signal this phase exists to collect.
  Striking is disabled at the rule level instead (max_strikes = 0, stall rule
  disabled).

  Watch, for ~2 weeks:
    * qBittorrent tag counts for `cleanuparr-unlinked` / `cleanuparr-dead`
    * Loki, container `cleanuparr`, for would-be strikes the thresholds suppress
  MEASURED 2026-08-09, first day of phase 0 (878 torrents):
    cleanuparr-unlinked  346     noHL  330     overlap  327
    * 19 tagged `cleanuparr-unlinked` but not `noHL`: ALL 19 have nlink=2 on
      disk, i.e. the *arr imported them and the tag is STALE. Cleanuparr's
      unlinked tag is add-only — it does not come off when the file later gains
      a hardlink. (Decisive case: a bare .mkv at nlink=2 with no nlink=1 file
      anywhere in the torrent still carries the tag, so no live scan could be
      re-deriving it.) qbit-manage does self-heal — `tag_nohardlinks` untags via
      check_previous_nohardlinks_tagged_torrents.
    * 3 tagged `noHL` but not `cleanuparr-unlinked`: the three Lidarr items the
      QueueCleaner logs as "skip | item is missing the content id". Downloads
      cleanuparr skips are never scanned for hardlinks either, so this is a
      blind spot noHL covers and cleanuparr does not.

  So KEEP `tag_nohardlinks` in qbit-manage permanently. The two tags are not
  redundant: `noHL` is the accurate, self-healing signal; `cleanuparr-unlinked`
  is a high-water mark of "was ever unlinked". Do not treat cleanuparr's tag as
  current state — see the phase 2b warning below.

  Exit criteria for phase 0: hand-audit a sample of `cleanuparr-dead` (0 so far)
  and confirm no unexpected strike traffic in Loki.

  ── Phase 1 — strikes on, still no file deletion ────────────────────────────
  Flip, in this file:
    queue_cleaner.failed_import.max_strikes      0 -> 3
    queue_cleaner.downloading_metadata_max_strikes  0 -> 5
    stall_rules[0].enabled                     false -> true
  and uncomment the `slow_rules` block below.

  Keep `delete_private_torrents_from_client = false` everywhere. Effect: a
  struck-out item is removed from the *arr queue and blocklisted, so the *arr
  re-searches. qBittorrent still holds every file — nothing is deleted. On an
  all-private setup the failure mode is a re-grab loop (same release grabbed
  repeatedly): watch for it, and answer it by raising max_strikes or adding an
  Exclude pattern, not by moving on to phase 2.

  ── Phase 2 — destructive ───────────────────────────────────────────────────
  Two independent pieces (2a and 2b below, both commented out at their
  insertion points). Enable ONE AT A TIME, with a couple of weeks between.

  Prerequisites, both:
    * `/var/lib/appdata`-style tmpfiles rule for `/mnt/media/orphaned`, owned by
      cfg.uid/cfg.gid — see the tmpfiles block at the bottom of this file.
    * Confirm the NFS export does not squash uid ${toString 5037}: the container
      must be able to unlink under /mnt/media. Test with one file by hand before
      trusting a rule with it.
    * qbit-manage's `rem_unregistered` / `rem_orphaned` / `share_limits` stay
      false. Cleanuparr owns removal; qbit-manage stays tag-only. Two owners of
      deletion is how torrents disappear for reasons nobody can reconstruct.

  Hardlink note that makes 2b safe: the *arr apps import by hardlink into
  /mnt/media/library, and cross-seed hardlinks into /mnt/media/cross-seed/links.
  `delete_source_files = true` drops the download-side link only — the library
  copy and any cross-seed sibling survive with a lower link count.
*/
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.cleanuparr = {
    aspect-name = "hosts.server.containers.cleanuparr";
    includes = [
      den.aspects.containers._.cleanuparr
      den.aspects.infra._.clients._.container-cleanuparr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.networks._.apps
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, lib, ... }:
      let
        cfg = config.containerConfigs.cleanuparr;
        cl = config.infra.clients;
        port = cl.container-cleanuparr.ports.default;
        apps = config.infra.vlans.apps;
        # Every category qBittorrent defines (see qbittorrent/_bittorrent.nix) —
        # the unlinked/dead scans are scoped to downloads the *arr apps own.
        categories = [
          "tv"
          "movies"
          "books"
          "audiobooks"
          "music"
          "misc"
          # seasonpackarr's packs. Every file in one is a hardlink to an episode
          # already held, so these can never read as unlinked — which also means
          # a pack that outlives its episodes will not surface here.
          "seasonpacks"
        ];
        arr = name: version: {
          inherit name;
          enabled = true;
          url = "http://${cl."container-${name}".ip}:${toString cl."container-${name}".ports.default}";
          api_key = "\${env.${lib.toUpper name}_APIKEY}";
          inherit version;
        };
      in
      {
        systemd.services."container@cleanuparr" = {
          wants = [ "container@postgresql.service" ];
          after = [ "container@postgresql.service" ];
        };

        containerConfigs.cleanuparr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = cl.container-cleanuparr.mac;
          localAddress = "${cl.container-cleanuparr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          inherit port;
          dbHost = cl.container-postgres.ip;
          dbPort = cl.container-postgres.ports.default;
          mediaDir = "/mnt/media";
          envFile = config.sops.templates."cleanuparr-env".path;
          configuratarr = {
            enable = true;
            waitForHealthy = true;
            settings.cleanuparr = {
              type = "cleanuparr-v1";
              url = "http://localhost:${toString port}";
              api_key = "\${env.CLEANUPARR_API_KEY}";

              general = {
                display_support_banner = false;
                # Not dry_run: dry run suppresses the unlinked/dead *tagging* too,
                # and those tags are the whole point of this observation phase.
                # Striking is disabled at the rule level instead (see below).
                dry_run = false;
                status_check_enabled = true;
                # A WAN outage makes every torrent look stalled at once. Gate the
                # jobs on reachability so an outage can't mass-strike the queue.
                connectivity_check_enabled = true;
                connectivity_check_urls = [
                  "https://1.1.1.1"
                  "https://cloudflare.com"
                ];
                http_max_retries = 3;
                http_timeout = 30;
                strike_inactivity_window_hours = 24;
                history_retention_days = 90;
                log = {
                  level = "Information";
                  # Vector already ships stdout to Loki; the rolling files under
                  # dataDir would be a second copy nobody reads.
                  rolling_size_mb = 0;
                };
                # Caddy fronts the UI: trust its forwarded client address and treat
                # the apps VLAN as local, so the dashboard needs no second login.
                auth = {
                  disable_auth_for_local_addresses = true;
                  trust_forwarded_headers = true;
                  trusted_networks = [ "${apps.subnet}/${toString apps.subnetPrefix}" ];
                };
              };

              # PHASE 0 — observe only. Tagging runs; nothing strikes, nothing is
              # removed from a queue, nothing is deleted. Striking out a private
              # torrent still pulls the item from the *arr queue and blocklists it,
              # which on an all-private setup means a re-grab of the same release —
              # so the strike thresholds stay at 0 until the tags have been reviewed.
              # Phase 1 flips: failed_import.max_strikes 3,
              # downloading_metadata_max_strikes 5, stall rule enabled.
              queue_cleaner = {
                enabled = true;
                use_advanced_scheduling = false;
                failed_import = {
                  max_strikes = 0; # 0 disables striking. Phase 1 -> 3
                  ignore_private = false;
                  delete_private = false;
                  skip_if_not_found_in_client = true;
                  # Exclude with an empty pattern list means "strike every failed
                  # import". Include is the stored default and Cleanuparr rejects it
                  # outright with no patterns ("At least one pattern must be
                  # specified when using the Include pattern mode").
                  pattern_mode = "Exclude";
                  patterns = [ ];
                  change_category = false;
                };
                downloading_metadata_max_strikes = 0; # Phase 1 -> 5
                # Private-tracker grabs routinely carry no content id; true would
                # strike every one of them.
                process_no_content_id = false;
              };
              download_cleaner = {
                enabled = true;
                use_advanced_scheduling = true;
                # The hourly default stats the whole media tree over NFS on every
                # run (unlinked = hardlink count, dead = per-torrent seeder poll).
                # /mnt/media is the same NFS mount qBittorrent seeds from, and it
                # is IO-op sensitive (see qbittorrent/_bittorrent.nix). Unlinked
                # and dead state don't move fast enough to pay that hourly.
                cron_expression = "0 0 */6 * * ?";
              };

              # Per-*arr override of the global failed-import threshold, pinned to
              # -1 (disabled) so an upstream default change can't quietly start
              # striking behind the queue_cleaner settings above.
              sonarr_config.failed_import_max_strikes = -1;
              radarr_config.failed_import_max_strikes = -1;
              lidarr_config.failed_import_max_strikes = -1;

              # Every tracker here is private. The malware blocklists target
              # public-tracker junk, and blacklist_sync exists to seed *arr
              # blocklists from those same lists — both are dead weight.
              malware_blocker.enabled = false;
              blacklist_sync.enabled = false;

              # Proactive search would fan out across four private indexers behind
              # one Prowlarr. cross-seed's daily sweep + rssCadence already cover
              # that ground, under a rate limit tuned for those trackers.
              seeker.search_enabled = false;

              # `version` only picks a client shape for Whisparr (see Cleanuparr's
              # ArrClientFactory); for these four it is the *arr API version.
              # readarr is deliberately absent: it is the one *arr with no
              # configuratarr config of its own, and the packaged 0.4.18 is from
              # the archived upstream. Books go through lazylibrarian.
              sonarr_instances = [ (arr "sonarr" 3) ];
              radarr_instances = [ (arr "radarr" 3) ];
              lidarr_instances = [ (arr "lidarr" 1) ];

              # Declared but inert in phase 0 — the rule exists so phase 1 is a
              # one-line flip rather than a new resource. slow_rules stay
              # undeclared until phase 1; the destructive resources
              # (seeding_rules, orphaned_files_config) until phase 2.
              stall_rules = [
                {
                  name = "stalled";
                  enabled = false; # Phase 1 -> true
                  max_strikes = 5;
                  privacy_type = "Both";
                  # Private-tracker torrents are struck and re-grabbed, never pulled
                  # out from under the client — H&R and seed-forever depend on them
                  # staying put.
                  delete_private_torrents_from_client = false;
                  change_category = false;
                  reset_strikes_on_progress = true;
                }
              ];

              # PHASE 1 — uncomment alongside the strike-threshold flips above.
              # Strikes only: a struck-out private torrent leaves the *arr queue
              # and gets blocklisted, but stays in qBittorrent with its files.
              #
              # min_completion_percentage = 5 skips torrents that never started
              # (those are the stall rule's job, not this one).
              # ignore_above_size exempts the large grabs where a slow tail is
              # normal rather than a fault.
              # ignore_while_alt_speed_active matters only if qBittorrent ever
              # gets a scheduler — it has no alt-speed limits configured today.
              #
              # slow_rules = [
              #   {
              #     name = "slow";
              #     enabled = true;
              #     max_strikes = 5;
              #     privacy_type = "Both";
              #     min_speed = "500KB";
              #     max_time_hours = 6;
              #     min_completion_percentage = 5;
              #     max_completion_percentage = 100;
              #     ignore_above_size = "50GB";
              #     delete_private_torrents_from_client = false;
              #     change_category = false;
              #     reset_strikes_on_progress = true;
              #     ignore_while_alt_speed_active = true;
              #   }
              # ];

              download_clients = [
                {
                  name = "qbittorrent";
                  enabled = true;
                  type_name = "qBittorrent";
                  protocol = "Torrent";
                  # No credentials: the media VLAN is in qBittorrent's
                  # authSubnetWhitelist. Paths need no translation either — /mnt/media
                  # is bind-mounted here at the same path qBittorrent reports.
                  host = "http://${cl.container-qbittorrent.ip}:${toString cl.container-qbittorrent.ports.webui}";
                  # Tag, never recategorise: a new category has no save path, so
                  # qBittorrent would relocate the data under autoTMM. A tag moves
                  # nothing — these two only mark downloads for a human to review.
                  unlinked_config = {
                    enabled = true;
                    use_tag = true;
                    target_category = "cleanuparr-unlinked";
                    inherit categories;
                  };
                  dead_torrent_config = {
                    enabled = true;
                    use_tag = true;
                    target_category = "cleanuparr-dead";
                    max_strikes = 5;
                    inherit categories;
                  };

                  # PHASE 2a — orphaned files. MOVES, never deletes (while
                  # purge_after_hours is null): anything under a scan directory
                  # that no torrent in this client claims is relocated to
                  # orphaned_directory for review.
                  #
                  # NEVER add /mnt/media/library to scan_directories. Those are
                  # the *arr hardlinks; qBittorrent has no torrent for any of
                  # them, so every single file would read as orphaned and the
                  # whole library would be moved out from under Jellyfin.
                  # Same reasoning excludes /mnt/media/inbox, /mnt/media/rips and
                  # /mnt/media/watch — not client-owned.
                  # /mnt/media/cross-seed/links is client-owned (cross-seed
                  # injects with that save path) so it would be safe to scan, but
                  # there is nothing to gain by it.
                  # orphaned_directory must not overlap any scan directory —
                  # the server rejects the config outright if it does.
                  #
                  # Review the move queue by hand for a few cycles, THEN set
                  # purge_after_hours = 720 (30d) to let it self-empty.
                  #
                  # orphaned_files_config = {
                  #   enabled = true;
                  #   scan_directories = [
                  #     "/mnt/media/download"
                  #     "/mnt/media/complete/tv"
                  #     "/mnt/media/complete/movies"
                  #     "/mnt/media/complete/books"
                  #     "/mnt/media/complete/audiobooks"
                  #     "/mnt/media/complete/music"
                  #     "/mnt/media/complete/misc"
                  #   ];
                  #   orphaned_directory = "/mnt/media/orphaned";
                  #   exclude_patterns = [
                  #     "*.nfo"
                  #     "*.parts"
                  #     ".DS_Store"
                  #   ];
                  #   # unpackerr extracts into these same trees; 48h keeps an
                  #   # in-flight extraction from being read as an orphan.
                  #   min_file_age_hours = 48;
                  #   purge_after_hours = null;
                  # };

                  # PHASE 2b — seeding rules. This DELETES the torrent and its
                  # files. It is the only thing here that reclaims disk, and the
                  # only thing that can cost an H&R.
                  #
                  # Enable ONE TRACKER AT A TIME, loosest H&R rules first, and
                  # confirm against that tracker's actual requirements before
                  # writing its numbers. The values below are placeholders.
                  #
                  # tracker_patterns matches announce-domain suffixes. Read the
                  # real domains off the torrents in qBittorrent first: a typo
                  # means the rule silently never fires, and an over-broad
                  # pattern means it fires on everything. The pairs below mirror
                  # the announce domains qbit-manage already tags on
                  # (qbittorrent/_qbit-manage.nix `tracker`).
                  #
                  # Invariants worth keeping:
                  #   min_seed_time >= the tracker's H&R requirement, in hours
                  #   max_seed_time = -1  (ratio+time gate only, never a hard clock)
                  #   min_seeders   >= 3  (never be the last seed of anything)
                  #   privacy_type = "Private"
                  # delete_source_files = true is safe here — see the hardlink
                  # note in the file header.
                  #
                  # DO NOT gate a rule on tags_any = [ "cleanuparr-unlinked" ].
                  # Measured on 2026-08-09: that tag is add-only — cleanuparr
                  # never removes it once the *arr imports the download. All 19
                  # torrents carrying it without noHL had nlink=2, i.e. imported
                  # fine. A deletion rule keyed on it would delete good, imported,
                  # seeding torrents. See the header for the full measurement.
                  #
                  # If a rule should target never-imported downloads, key it on
                  # qbit-manage's `noHL` instead — that one is recomputed and
                  # removed every pass:
                  #   tags_any = [ "noHL" ];
                  #
                  # seeding_rules = [
                  #   {
                  #     name = "torrentleech";
                  #     inherit categories;
                  #     tracker_patterns = [ "torrentleech.org" "tleechreload.org" ];
                  #     privacy_type = "Private";
                  #     max_ratio = 2.0;
                  #     min_seed_time = 240.0;
                  #     max_seed_time = -1;
                  #     min_seeders = 3;
                  #     delete_source_files = true;
                  #   }
                  #   {
                  #     name = "myanonamouse";
                  #     inherit categories;
                  #     tracker_patterns = [ "myanonamouse.net" ];
                  #     privacy_type = "Private";
                  #     max_ratio = 2.0;
                  #     # MAM's H&R rules are the strictest of the five — 336h
                  #     # (14d) is a floor, verify before enabling.
                  #     min_seed_time = 336.0;
                  #     max_seed_time = -1;
                  #     min_seeders = 3;
                  #     delete_source_files = true;
                  #   }
                  #   # digitalcore / pretome (preto.me) / retrotoon likewise.
                  # ];
                }
              ];
            };
          };
          bindMounts = {
            "/var/lib/cleanuparr" = {
              hostPath = "/var/lib/appdata/cleanuparr";
              isReadOnly = false;
            };
            # Unlinked/orphaned download handling stats the real tree qBittorrent
            # and the *arr apps write to.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = false;
            };
          };
          nameservers = [
            cl.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${cl.container-loki.ip}:${toString cl.container-loki.ports.default}";
            prometheusAddress = "http://${cl.container-prometheus.ip}:${toString cl.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${cl.container-tempo.ip}:${toString cl.container-tempo.ports.otlp-http}/v1/traces";
          };
        };

        systemd.tmpfiles.rules = [
          "d /var/lib/appdata/cleanuparr 0700 ${toString cfg.uid} ${toString cfg.gid} - -"
          "Z /var/lib/appdata/cleanuparr 0700 ${toString cfg.uid} ${toString cfg.gid} - -"
          # PHASE 2a — landing area for orphaned_files_config. Uncomment together
          # with that block. No Z line: /mnt/media is the NFS share, and a
          # recursive chown across it is not something to run on every activation.
          # "d /mnt/media/orphaned 0755 ${toString cfg.uid} ${toString cfg.gid} - -"
        ];
      };
  };
}
