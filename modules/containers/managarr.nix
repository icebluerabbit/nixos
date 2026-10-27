# managarr — state recorder for the download and library stack
# packages/go/managarr
#
# Replaces cleanuparr and qbit-manage. Both decided what was safe to delete by
# counting hardlinks, which is only correct when the *arr imported by hardlink —
# wrong for anything extracted from an archive, and wrong again for anything
# cross-seed had linked. An *arr import record states the mapping outright: it
# carries the download id next to the file it produced.
#
# Mostly reads. It polls each service's API into PostgreSQL on a timer, and
# serves a webhook endpoint per sender under /hook/ for the events those APIs do
# not retain.
#
# It also acts, on five timers, each of which defaults to reporting rather than
# doing and is enabled separately:
#
#   cleanup  deletes torrents a tracker has unregistered, with their data
#   orphans  deletes Sonarr/Radarr entries no torrent provides any more
#   reap     reports data and rows nothing claims; deletes nothing
#   tags     REPLACES every tag on every torrent with what the model says
#   packs    records which episodes each assembled season pack was built from
#
# The first two are irreversible. tags is not — a wrong set is corrected by the
# next hour's pass — but it is the only one that touches every torrent. packs
# writes associations and nothing else.
#
# The reconcile needs API keys, so envFile is required for any source that has
# one; the service refuses to start with a URL configured and its key missing,
# rather than leaving a table silently empty.
_: {
  den.aspects.containers._.managarr = {
    aspect-name = "containers.managarr";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      let
        hostConfig = config;
        # Resolved from HOST pkgs: the container is built from its own nixpkgs
        # instance, which carries no homelab overlay. The store path reaches
        # the container's closure through ExecStart.
        managarrExe = lib.getExe pkgs.homelab.managarr;
      in
      {
        options.containerConfigs.managarr =
          (import ./_options.nix {
            inherit lib;
            defaultPort = 9911;
            defaultUid = 5044;
            defaultGid = 5044;
          })
          // {
            metricsPort = lib.mkOption {
              type = lib.types.port;
              default = 9090;
              description = "Loopback port for the Prometheus endpoint the Vector sidecar scrapes";
            };
            shutdownTimeout = lib.mkOption {
              type = lib.types.str;
              default = "10s";
              description = "Grace period for in-flight webhook requests on stop";
            };
            # Postgres rather than SQLite: the model is relational across a
            # dozen entities, the host already runs the cluster for six *arr
            # tenants, and pg_hba grants this container `trust` by IP — so
            # there is no credential to carry.
            dbHost = lib.mkOption {
              type = lib.types.str;
              default = "127.0.0.1";
              description = "PostgreSQL host";
            };
            dbPort = lib.mkOption {
              type = lib.types.port;
              default = 5432;
              description = "PostgreSQL port";
            };
            dbName = lib.mkOption {
              type = lib.types.str;
              default = "managarr";
              description = "PostgreSQL database name";
            };
            dbUser = lib.mkOption {
              type = lib.types.str;
              default = "managarr";
              description = "PostgreSQL role; owns dbName";
            };
            qbittorrentUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                qBittorrent WebUI base URL. Empty disables POST /reconcile.
                No credentials: the media VLAN is in qBittorrent's
                authSubnetWhitelist.
              '';
            };
            reconcileInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                systemd OnCalendar for the reconcile timer; null disables it.
                A pass stats every file of every torrent over the NFS mount,
                so this is deliberately infrequent.
              '';
            };
            sonarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "Sonarr base URL; empty skips it in the reconcile";
            };
            radarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "Radarr base URL; empty skips it in the reconcile";
            };
            lidarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "Lidarr base URL; empty skips it in the reconcile";
            };
            sonarrSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                managarr-sonarr base URL. The sidecar serves the same history
                Sonarr does, with a since-cursor Sonarr itself does not offer —
                /history takes a page and a sort and nothing else, so without
                one every hourly pass replays all 28,000 records to find the
                handful that are new.

                Empty falls back to reading Sonarr directly. A real fallback
                rather than a degraded mode: correct, just wasteful, so a
                sidecar being down costs the cursor and not the pass.
              '';
            };
            radarrSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "managarr-radarr base URL; empty reads Radarr's history directly";
            };
            lidarrSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "managarr-lidarr base URL; empty reads Lidarr's history directly";
            };
            autobrrSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "managarr-autobrr base URL; empty reads autobrr directly";
            };
            prowlarrSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "managarr-prowlarr base URL; empty reads Prowlarr directly";
            };
            slskdSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "managarr-slskd base URL; empty reads slskd directly";
            };
            beetsSidecarUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "managarr-beets base URL; empty reads beets directly";
            };
            cleanupInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                systemd OnCalendar for the cleanup timer; null disables it.

                Removes torrents a tracker has unregistered — they can never be
                announced again, so seeding one moves no bytes and holds a slot
                forever. The data goes with them: an *arr import and a
                cross-seed inject are hardlinks, so unlinking the client's copy
                leaves theirs and the bytes only go when nothing else has a
                link.

                Daily is plenty. A dead torrent is not getting deader.
              '';
            };
            cleanupApply = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = ''
                Whether the cleanup timer actually deletes.

                Off by default, and deliberately so: the evidence for deleting
                is a string a tracker sent, the action is irreversible, and a
                timer that has just been switched on should report what it
                would have done until someone has read the log.
              '';
            };
            orphanInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                systemd OnCalendar for the orphan timer; null disables it.

                Deletes Sonarr and Radarr library entries no torrent provides
                any more — the other half of what the cleanup starts. Cleanup
                removes an unregistered torrent and the library file survives
                it, because an import is a hardlink; this removes the file once
                nothing is seeding it under any infohash.

                Whether anything still provides it comes from cross-seed's own
                record of which infohashes carry the same content, so a file
                three other trackers are seeding is never touched.

                Sonarr and Radarr only. Music arrives through Soulseek and beets
                as well as torrents, so a Lidarr album with no live torrent is an
                ordinary album.
              '';
            };
            orphanApply = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = ''
                Whether the orphan timer actually deletes.

                Off by default, and more emphatically than cleanupApply: this
                one unlinks MEDIA FILES. The cleanup removes a torrent's copy
                and usually leaves the bytes behind another hardlink; there is
                no second link left by the time this fires.

                Leave it off and read what the dry run reports. It answers with
                the full list of files rather than a count, which is the only
                form worth approving.
              '';
            };
            reapInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                systemd OnCalendar for the reap sweep; null disables it.

                Removes everything else the stack has stranded: download data
                whose torrent is gone — the usual result of removing one by
                hand without its data — dead cross-seed link trees, and
                abandoned partials.

                Also reports two things it cannot act on, because they are the
                inverse case of a row claiming a file that is not there:
                torrents whose data has vanished, and *arr rows pointing at
                missing files. Those are diagnostics; acting on them would mean
                editing the database to hide a problem instead of fixing it.
              '';
            };
            reapApply = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = ''
                Whether the reap sweep actually deletes what it finds.

                Off by default. The evidence for removing an entry is the
                ABSENCE of a claim on it, and absence is exactly what a stale or
                partial read produces — a sweep that once ran during a reconcile
                reported four cross-seed injections from twenty minutes earlier
                as abandoned.

                Two defences against that: the claimed set is read from the
                torrent client rather than the database, and it is read again
                immediately before anything is unlinked, since the directory
                walk in between can take minutes over NFS. An entry claimed in
                that window is spared and counted.

                Note the deleted byte figure overstates what is reclaimed. These
                trees are largely hardlinks, so space returns only where no
                other link remains.
              '';
            };
            completeDir = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                Download tree, laid out as <category>/<entry>. Empty skips the
                reap sweep's download-data check.
              '';
            };
            linksDir = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = "cross-seed link tree as <tracker>/<entry>; empty skips that check";
            };
            incompleteDir = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                Partial-download root; empty skips that check.

                Normally empty here. The directory belongs to the torrent
                client's container and is not mounted into this one, so the
                sweep records the check as skipped rather than reporting a
                clean result it did not earn.
              '';
            };
            libraryDirs = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = ''
                Library roots, used only to bound the missing-file check — a
                row pointing somewhere unmounted is skipped rather than
                reported as missing.
              '';
            };
            reapCategories = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = ''
                Download categories torrents actually manage.

                Load-bearing. The same tree holds soulseek/, rips/ and
                youtube/, which no torrent ever claimed — "unclaimed" says
                nothing about those, and treating them as leftovers would
                propose deleting the entire Soulseek library.
              '';
            };
            tagInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                systemd OnCalendar for the tag projection; null disables it.

                Projects managarr's view onto qBittorrent's tags, REPLACING
                what each torrent carries. Hourly: the facts behind the tags —
                what a tracker says, whether an *arr has imported it — change
                on that scale, and a replaced set means a stale tag cannot
                outlive the thing that wrote it.

                Replacing is the point. Three tools wrote tags here and two were
                removed; their tags froze on the day they stopped and 43% of
                torrents ended up with none at all. A single authority stating
                the whole set is the only arrangement where that cannot recur.
              '';
            };
            packsDir = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                seasonpackarr preImportPath. Content under it is a season pack
                seasonpackarr ASSEMBLED from episodes already in the client,
                rather than one that was downloaded.

                A third origin: without it those torrents are indistinguishable
                from ordinary grabs, carrying a tracker and a library state like
                anything else with nothing saying where they came from.
              '';
            };
            packsInterval = lib.mkOption {
              type = lib.types.nullOr lib.types.str;
              default = null;
              description = ''
                systemd OnCalendar for the pack-source pass; null disables it.

                Records which episode torrents each assembled season pack was
                built from, by resolving the names inside the pack directory to
                client torrents. seasonpackarr creates that association and then
                forgets it — a pack is otherwise indistinguishable from
                something downloaded, and its episodes from episodes nothing
                depends on.

                It matters for deletion: a pack and its episodes are the same
                bytes under two names, so acting on one while the other seeds
                breaks the survivor.

                Hourly alongside the tags, since both read the same snapshot.
              '';
            };
            packsApply = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = ''
                Whether the pack-source pass writes the associations it finds.

                Off by default. It creates and deletes nothing else, so a wrong
                row is corrected by the next pass — but a misattributed pack is
                worth seeing before anything downstream reasons about it.
              '';
            };
            seedRequirements = lib.mkOption {
              type = lib.types.attrsOf (
                lib.types.submodule {
                  options = {
                    days = lib.mkOption {
                      type = lib.types.number;
                      default = 0;
                      description = ''
                        Minimum seeding time in days. 0 means the tracker sets
                        none and only ratio can clear the obligation.
                      '';
                    };
                    ratio = lib.mkOption {
                      type = lib.types.number;
                      default = 0;
                      description = ''
                        Minimum share ratio. 0 means there is no ratio path and
                        only time can clear the obligation.
                      '';
                    };
                    timeNeedsComplete = lib.mkOption {
                      type = lib.types.bool;
                      default = true;
                      description = ''
                        Whether seeding time only accrues once the torrent is
                        fully downloaded.

                        Trackers genuinely differ here. TorrentLeech, DarkPeers,
                        YU-Scene and BitPorn all say time starts at 100%;
                        DigitalCore says the opposite outright — "seeding means
                        making a torrent available for uploading, complete or
                        not" — and SeedPool says nothing, exempting small
                        partials instead.

                        True by default: a partial torrent banking time toward
                        an obligation it is not discharging is the dangerous
                        direction to be wrong in.
                      '';
                    };
                    exemptBelowProgress = lib.mkOption {
                      type = lib.types.number;
                      default = 0;
                      description = ''
                        Download fraction below which the tracker raises no
                        obligation at all.

                        DigitalCore, SeedPool and DarkPeers all use 0.10;
                        YU-Scene uses 0.15. Note this is a different mechanic
                        from timeNeedsComplete and the two compose: DarkPeers
                        has both, so a torrent between 10% and 100% there is
                        charged but cannot discharge by time, leaving ratio as
                        its only route.

                        0 means no exemption, which is the safe default — a
                        tracker that has not said it forgives small partials
                        has not forgiven them.
                      '';
                    };
                  };
                }
              );
              default = { };
              description = ''
                What each tracker demands before a torrent may be stopped,
                keyed by announce host.

                Either limb clears it, not both: the trackers word it as "seed
                to 1:1 OR seed for the minimum time", and requiring both would
                hold torrents forever for no reason.

                ALWAYS CONFIGURE THE STRICTEST TIER. Where a tracker scales the
                requirement by user class — TorrentLeech runs 10 days for
                Registered down to none for VIP — use the harshest number.
                Being wrong that way costs disk; being wrong the other way
                costs the account.

                A host with no entry tags seed::unknown rather than being
                assumed safe.
              '';
            };
            preserveTags = lib.mkOption {
              type = lib.types.listOf lib.types.str;
              default = [ ];
              description = ''
                Tags another tool owns and reads back, kept on any torrent
                already carrying one instead of being replaced.

                The single exception to replacing tags wholesale, and it exists
                for cross-seed. Its preFilter decides what to search FROM, and
                one of its tests is whether the torrent carries its own tag:

                  if (searchee.tags?.includes(TORRENT_TAG))

                TORRENT_TAG is the hardcoded string "cross-seed". Version
                6.13.7 has no setting that disables tagging — every branch of
                getTagsForNewTorrent returns it and createTag re-registers it
                at startup — so the tag cannot be turned off at the source.
                Stripping it makes cross-seed stop recognising its own injects
                and search from them, which burns indexer budget against
                excludeRecentSearch and produces cross-seeds of cross-seeds.

                Preserved rather than emitted, so the tag stays cross-seed's to
                say. A torrent whose tag was already lost does not regain one
                here; only cross-seed re-applying it restores that.
              '';
            };
            trackerTags = lib.mkOption {
              type = lib.types.attrsOf lib.types.str;
              default = { };
              description = ''
                Announce host to the value its tracker:: tag carries.

                Declared rather than derived, because one tracker announces
                under several hostnames whose domains share no common part:
                tracker.torrentleech.org and tracker.tleechreload.org are one
                site, as are tracker.digitalcore.club and
                trackerprxy.digitalcore.club. Without the mapping a torrent
                carries two tracker tags for one tracker.

                An unmapped host tags with its own hostname, so a new tracker is
                visibly ugly rather than silently untagged.
              '';
            };
            statsInterval = lib.mkOption {
              type = lib.types.str;
              default = "1m";
              description = ''
                How often the domain gauges are recomputed from the database.

                Independent of reconcileInterval, which is hourly because a
                pass stats every file over NFS. These are indexed aggregates
                costing milliseconds, so tying them to the pass would discard
                every trend finer than an hour.
              '';
            };
            slskdUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                slskd base URL; empty skips it. No credentials: slskd runs
                with authentication.disabled and is gated at Caddy.
              '';
            };
            bazarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                Bazarr base URL; empty skips it. Requires BAZARR_APIKEY in
                envFile. Bazarr is the only source of subtitle state, and the
                only source of an absence: which languages are still WANTED
                cannot be observed on disk, because a missing subtitle leaves
                nothing behind.
              '';
            };
            crossSeedUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                Base URL of managarr-crossseed, the read-only sidecar that runs
                inside cross-seed's container and serves its decision log;
                empty skips the decision replay.

                The sidecar exists because cross-seed's own API does not: it
                offers /api/ping, /api/announce and /api/webhook, and the
                webhook fires only on a MATCH. The rejections are four fifths
                of the record and the only answer to why something was not
                cross-seeded, and they live nowhere but the database file.
              '';
            };
            beetsUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                Base URL of beets' web plugin; empty skips the tag pass.

                beets is where the music loop closes: its mb_releasegroupid is
                what Lidarr calls foreignAlbumId, and its item paths carry the
                inodes that tie a library file back to the Soulseek download it
                came from. The plugin needs include_paths for the latter.
              '';
            };
            autobrrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                autobrr base URL; empty skips it. Requires AUTOBRR_APIKEY in
                envFile — managarr refuses to start with a URL and no key,
                because autobrr is the only source of announcements and of the
                per-action rejections that say why an approved release never
                reached an *arr.
              '';
            };
            autobrrFull = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = ''
                Walk autobrr's entire release history on every pass instead of
                stopping at the first page holding nothing new. Needed once to
                backfill; a scheduled pass leaves it off, since a full walk of
                a six-figure corpus takes minutes and a incremental one takes
                seconds.
              '';
            };
            prowlarrUrl = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                Prowlarr base URL; empty skips it. Requires PROWLARR_APIKEY in
                envFile. Prowlarr proxies every indexer query, so its history is
                the only record of what was searched for as opposed to what was
                grabbed.
              '';
            };
            prowlarrFull = lib.mkOption {
              type = lib.types.bool;
              default = false;
              description = "Walk Prowlarr's whole history rather than stopping at the first page holding nothing new. Needed once, to backfill.";
            };
            ripsDir = lib.mkOption {
              type = lib.types.str;
              default = "";
              description = ''
                CD-rip staging root, laid out as <artist>/<album>. Empty skips
                the scan. This is the staging copy rather than the library: the
                whipper log carrying the disc id and AccurateRip result does
                not follow the audio when beets hands it to Lidarr, so staging
                is the only place a rip's provenance survives.
              '';
            };
            envFile = lib.mkOption {
              type = lib.types.nullOr lib.types.path;
              default = null;
              description = ''
                Host path to an env file holding the API keys the reconcile
                needs: SONARR_APIKEY, RADARR_APIKEY, LIDARR_APIKEY,
                BAZARR_APIKEY, AUTOBRR_APIKEY and PROWLARR_APIKEY. Passed
                through the environment rather than argv so the keys never
                appear in the process table.
              '';
            };
          };

        config = {
          containers.managarr =
            let
              cfg = hostConfig.containerConfigs.managarr;
            in
            {
              inherit (cfg) autoStart;
              ephemeral = true;
              privateNetwork = true;
              inherit (cfg) hostBridge;
              localMacAddress = cfg.macAddress;
              inherit (cfg) localAddress;
              bindMounts =
                cfg.bindMounts
                // (lib.optionalAttrs (cfg.envFile != null) {
                  "/run/secrets/managarr-env" = {
                    hostPath = cfg.envFile;
                    isReadOnly = true;
                  };
                });

              config =
                { pkgs, lib, ... }:
                {
                  imports = [ (import ./_base.nix { inherit lib cfg; }) ];

                  # The reconcile is a timer hitting the daemon's own endpoint
                  # rather than a ticker inside the process: a pass stats every
                  # file of every torrent over NFS and has no business running
                  # in the same goroutine budget as a webhook handler. The
                  # endpoint answers 202 at once and 409 if a pass is already
                  # running, so an overlapping tick is a no-op.
                  systemd = {
                    services = {

                      managarr-reconcile = lib.mkIf (cfg.reconcileInterval != null && cfg.qbittorrentUrl != "") {
                        description = "Re-read full download client state into managarr";
                        after = [ "managarr.service" ];
                        requires = [ "managarr.service" ];
                        path = [ pkgs.curl ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "managarr";
                          Group = "managarr";
                        };
                        # 409 is a success here, not a failure: it means a pass
                        # is already running and this tick is the documented
                        # no-op. `curl -sf` exits 22 on it, which would put the
                        # unit into `failed` and surface in every health check
                        # for entirely normal behaviour.
                        script = ''
                          code=$(curl -s -o /dev/null -w '%{http_code}' \
                            -X POST http://127.0.0.1:${toString cfg.port}/reconcile)
                          case "$code" in
                            202) echo "reconcile started" ;;
                            409) echo "reconcile already running; skipping" ;;
                            *)   echo "reconcile request failed: HTTP $code" >&2; exit 1 ;;
                          esac
                        '';
                      };
                      # Remove torrents their tracker has unregistered.
                      #
                      # Runs AFTER a reconcile rather than on its own schedule: the
                      # endpoint acts on the tracker state the last pass recorded,
                      # so firing it on an independent clock would sometimes delete
                      # against observations an hour old for no benefit.
                      #
                      # cleanupApply is the safety. Off, this reports what it would
                      # delete and touches nothing — which is what a newly enabled
                      # timer should do until someone has read the log.
                      managarr-cleanup = lib.mkIf (cfg.cleanupInterval != null && cfg.qbittorrentUrl != "") {
                        description = "Delete torrents their tracker has unregistered";
                        after = [ "managarr.service" ];
                        requires = [ "managarr.service" ];
                        path = [ pkgs.curl ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "managarr";
                          Group = "managarr";
                        };
                        script = ''
                          code=$(curl -s -o /dev/null -w '%{http_code}' \
                            -X POST "http://127.0.0.1:${toString cfg.port}/cleanup?apply=${
                              if cfg.cleanupApply then "true" else "false"
                            }")
                          case "$code" in
                            200) echo "cleanup completed" ;;
                            501) echo "cleanup not configured; skipping" ;;
                            *)   echo "cleanup request failed: HTTP $code" >&2; exit 1 ;;
                          esac
                        '';
                      };
                      # Delete library entries no torrent provides any more.
                      #
                      # Ordered after the cleanup, because it reads what the cleanup
                      # wrote: a library file only becomes a candidate once managarr
                      # has recorded deleting the torrent it came from. Running them
                      # the other way round would make every orphan wait a full
                      # cycle to be noticed.
                      #
                      # The response BODY is logged, not just the status. A dry run
                      # answers with the list of files it would delete, and that
                      # list is the entire reason to run it — a count in the journal
                      # would be useless for deciding whether to apply.
                      managarr-orphans =
                        lib.mkIf (cfg.orphanInterval != null && (cfg.sonarrUrl != "" || cfg.radarrUrl != ""))
                          {
                            description = "Delete library entries no torrent provides any more";
                            after = [
                              "managarr.service"
                              "managarr-cleanup.service"
                            ];
                            requires = [ "managarr.service" ];
                            path = [
                              pkgs.curl
                              pkgs.coreutils
                            ];
                            serviceConfig = {
                              Type = "oneshot";
                              User = "managarr";
                              Group = "managarr";
                            };
                            script = ''
                              out=$(mktemp)
                              trap 'rm -f "$out"' EXIT
                              code=$(curl -s -o "$out" -w '%{http_code}' \
                                -X POST "http://127.0.0.1:${toString cfg.port}/orphans?apply=${
                                  if cfg.orphanApply then "true" else "false"
                                }")
                              case "$code" in
                                200) echo "orphans completed"; cat "$out" ;;
                                501) echo "orphans not configured; skipping" ;;
                                *)   echo "orphans request failed: HTTP $code" >&2
                                     cat "$out" >&2
                                     exit 1 ;;
                              esac
                            '';
                          };
                      # Report everything else the stack has stranded.
                      #
                      # Read-only, so there is no apply option and no guard beyond
                      # having somewhere to look. Ordered last: it compares what the
                      # database says is held against what is on disk, and both of
                      # the sweeps before it change that comparison.
                      #
                      # The response BODY is logged. The whole product of this unit
                      # is the list it returns.
                      managarr-reap = lib.mkIf (cfg.reapInterval != null && cfg.completeDir != "") {
                        description = "Report data and library rows nothing claims";
                        after = [
                          "managarr.service"
                          "managarr-cleanup.service"
                          "managarr-orphans.service"
                        ];
                        requires = [ "managarr.service" ];
                        path = [
                          pkgs.curl
                          pkgs.coreutils
                        ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "managarr";
                          Group = "managarr";
                        };
                        script = ''
                          out=$(mktemp)
                          trap 'rm -f "$out"' EXIT
                          code=$(curl -s -o "$out" -w '%{http_code}' \
                            -X POST "http://127.0.0.1:${toString cfg.port}/reap?apply=${
                              if cfg.reapApply then "true" else "false"
                            }")
                          case "$code" in
                            200) echo "reap completed"; cat "$out" ;;
                            501) echo "reap not configured; skipping" ;;
                            *)   echo "reap request failed: HTTP $code" >&2
                                 cat "$out" >&2
                                 exit 1 ;;
                          esac
                        '';
                      };
                      # Project managarr's view onto the client's tags.
                      #
                      # Hourly, and ordered after the reconcile it reads from: the
                      # tags are computed entirely from what the last pass recorded,
                      # so running on an independent clock would tag against
                      # observations an hour old for no benefit.
                      #
                      # No dry-run switch, unlike the other sweeps. The whole set is
                      # recomputed and replaced every hour, so a wrong tag survives
                      # at most that long and costs nothing meanwhile — while a flag
                      # left off would just mean the tags stayed stale, which is the
                      # state this exists to end.
                      #
                      # The endpoint still defaults to a dry run, so a bare POST by
                      # hand stays inert; only this unit asks it to apply.
                      managarr-tags = lib.mkIf (cfg.tagInterval != null && cfg.qbittorrentUrl != "") {
                        description = "Project managarr's view onto qBittorrent's tags";
                        after = [
                          "managarr.service"
                          "managarr-reconcile.service"
                        ];
                        requires = [ "managarr.service" ];
                        path = [
                          pkgs.curl
                          pkgs.coreutils
                        ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "managarr";
                          Group = "managarr";
                        };
                        script = ''
                          out=$(mktemp)
                          trap 'rm -f "$out"' EXIT
                          code=$(curl -s -o "$out" -w '%{http_code}' \
                            -X POST "http://127.0.0.1:${toString cfg.port}/tags?apply=true")
                          case "$code" in
                            200) echo "tags completed"; cat "$out" ;;
                            501) echo "tags not configured; skipping" ;;
                            *)   echo "tags request failed: HTTP $code" >&2
                                 cat "$out" >&2
                                 exit 1 ;;
                          esac
                        '';
                      };
                      # Record which episodes each assembled pack was built from.
                      #
                      # After the reconcile, which is what supplies the names it
                      # resolves against: a pack assembled since the last pass has
                      # no client_torrents row yet, and resolving it would report
                      # every one of its episodes as unmatched.
                      managarr-packs = lib.mkIf (cfg.packsInterval != null && cfg.packsDir != "") {
                        description = "Record which episodes each season pack was assembled from";
                        after = [
                          "managarr.service"
                          "managarr-reconcile.service"
                        ];
                        requires = [ "managarr.service" ];
                        path = [
                          pkgs.curl
                          pkgs.coreutils
                        ];
                        serviceConfig = {
                          Type = "oneshot";
                          User = "managarr";
                          Group = "managarr";
                        };
                        script = ''
                          out=$(mktemp)
                          trap 'rm -f "$out"' EXIT
                          code=$(curl -s -o "$out" -w '%{http_code}' \
                            -X POST "http://127.0.0.1:${toString cfg.port}/packs?apply=${
                              if cfg.packsApply then "true" else "false"
                            }")
                          case "$code" in
                            200) echo "packs completed"; cat "$out" ;;
                            501) echo "packs not configured; skipping" ;;
                            *)   echo "packs request failed: HTTP $code" >&2
                                 cat "$out" >&2
                                 exit 1 ;;
                          esac
                        '';
                      };

                      managarr = {
                        description = "Record events from the download and library stack";
                        wantedBy = [ "multi-user.target" ];
                        after = [ "network.target" ];
                        serviceConfig = {
                          Type = "simple";
                          User = "managarr";
                          Group = "managarr";
                          ExecStart = lib.concatStringsSep " " (
                            [
                              managarrExe
                              "--addr :${toString cfg.port}"
                              "--shutdown-timeout ${cfg.shutdownTimeout}"
                              "--metrics.addr 127.0.0.1:${toString cfg.metricsPort}"
                              "--db-dsn postgres://${cfg.dbUser}@${cfg.dbHost}:${toString cfg.dbPort}/${cfg.dbName}?sslmode=disable"
                            ]
                            ++ lib.optional (cfg.qbittorrentUrl != "") "--qbittorrent-url ${cfg.qbittorrentUrl}"
                            ++ lib.optional (cfg.sonarrUrl != "") "--sonarr-url ${cfg.sonarrUrl}"
                            ++ lib.optional (cfg.radarrUrl != "") "--radarr-url ${cfg.radarrUrl}"
                            ++ lib.optional (cfg.lidarrUrl != "") "--lidarr-url ${cfg.lidarrUrl}"
                            ++ lib.optional (cfg.sonarrSidecarUrl != "") "--sonarr-sidecar-url ${cfg.sonarrSidecarUrl}"
                            ++ [ "--stats-interval ${cfg.statsInterval}" ]
                            ++ lib.optional (cfg.autobrrSidecarUrl != "") "--autobrr-sidecar-url ${cfg.autobrrSidecarUrl}"
                            ++ lib.optional (cfg.prowlarrSidecarUrl != "") "--prowlarr-sidecar-url ${cfg.prowlarrSidecarUrl}"
                            ++ lib.optional (cfg.slskdSidecarUrl != "") "--slskd-sidecar-url ${cfg.slskdSidecarUrl}"
                            ++ lib.optional (cfg.beetsSidecarUrl != "") "--beets-sidecar-url ${cfg.beetsSidecarUrl}"
                            ++ lib.optional (cfg.radarrSidecarUrl != "") "--radarr-sidecar-url ${cfg.radarrSidecarUrl}"
                            ++ lib.optional (cfg.lidarrSidecarUrl != "") "--lidarr-sidecar-url ${cfg.lidarrSidecarUrl}"
                            ++ lib.optional (cfg.slskdUrl != "") "--slskd-url ${cfg.slskdUrl}"
                            ++ lib.optional (cfg.bazarrUrl != "") "--bazarr-url ${cfg.bazarrUrl}"
                            ++ lib.optional (cfg.autobrrUrl != "") "--autobrr-url ${cfg.autobrrUrl}"
                            ++ lib.optional cfg.autobrrFull "--autobrr-full"
                            ++ lib.optional (cfg.prowlarrUrl != "") "--prowlarr-url ${cfg.prowlarrUrl}"
                            ++ lib.optional cfg.prowlarrFull "--prowlarr-full"
                            ++ lib.optional (cfg.crossSeedUrl != "") "--cross-seed-url ${cfg.crossSeedUrl}"
                            ++ lib.optional (cfg.beetsUrl != "") "--beets-url ${cfg.beetsUrl}"
                            ++ lib.optional (cfg.ripsDir != "") "--rips-dir ${cfg.ripsDir}"
                            ++ lib.optional (cfg.completeDir != "") "--complete-dir ${cfg.completeDir}"
                            ++ lib.optional (cfg.linksDir != "") "--links-dir ${cfg.linksDir}"
                            ++ lib.optional (cfg.incompleteDir != "") "--incomplete-dir ${cfg.incompleteDir}"
                            ++ lib.optional (cfg.packsDir != "") "--packs-dir ${cfg.packsDir}"
                            ++ lib.optional (
                              cfg.libraryDirs != [ ]
                            ) "--library-dirs ${lib.concatStringsSep "," cfg.libraryDirs}"
                            ++ lib.optional (
                              cfg.reapCategories != [ ]
                            ) "--reap-categories ${lib.concatStringsSep "," cfg.reapCategories}"
                            ++ lib.optional (
                              cfg.preserveTags != [ ]
                            ) "--preserve-tags ${lib.concatStringsSep "," cfg.preserveTags}"
                            ++ lib.optional (
                              cfg.seedRequirements != { }
                            ) "--seed-requirements '${builtins.toJSON cfg.seedRequirements}'"
                            ++ lib.optional (cfg.trackerTags != { }) (
                              "--tracker-tags "
                              + lib.concatStringsSep "," (lib.mapAttrsToList (host: value: "${host}=${value}") cfg.trackerTags)
                            )
                          );
                          EnvironmentFile = lib.mkIf (cfg.envFile != null) "/run/secrets/managarr-env";
                          Restart = "on-failure";
                          RestartSec = 5;
                        };
                      };
                    };
                    timers = {

                      managarr-reconcile = lib.mkIf (cfg.reconcileInterval != null && cfg.qbittorrentUrl != "") {
                        description = "Periodic managarr reconcile";
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnCalendar = cfg.reconcileInterval;
                          Persistent = true;
                          RandomizedDelaySec = "5m";
                        };
                      };

                      managarr-cleanup = lib.mkIf (cfg.cleanupInterval != null && cfg.qbittorrentUrl != "") {
                        description = "Periodic managarr cleanup";
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnCalendar = cfg.cleanupInterval;
                          Persistent = true;
                          RandomizedDelaySec = "10m";
                        };
                      };

                      managarr-orphans =
                        lib.mkIf (cfg.orphanInterval != null && (cfg.sonarrUrl != "" || cfg.radarrUrl != ""))
                          {
                            description = "Periodic managarr orphan sweep";
                            wantedBy = [ "timers.target" ];
                            timerConfig = {
                              OnCalendar = cfg.orphanInterval;
                              Persistent = true;
                              RandomizedDelaySec = "10m";
                            };
                          };

                      managarr-reap = lib.mkIf (cfg.reapInterval != null && cfg.completeDir != "") {
                        description = "Periodic managarr reap report";
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnCalendar = cfg.reapInterval;
                          Persistent = true;
                          RandomizedDelaySec = "10m";
                        };
                      };

                      managarr-tags = lib.mkIf (cfg.tagInterval != null && cfg.qbittorrentUrl != "") {
                        description = "Periodic managarr tag projection";
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnCalendar = cfg.tagInterval;
                          Persistent = true;
                          RandomizedDelaySec = "5m";
                        };
                      };

                      managarr-packs = lib.mkIf (cfg.packsInterval != null && cfg.packsDir != "") {
                        description = "Periodic managarr pack-source resolution";
                        wantedBy = [ "timers.target" ];
                        timerConfig = {
                          OnCalendar = cfg.packsInterval;
                          Persistent = true;
                          RandomizedDelaySec = "5m";
                        };
                      };
                    };
                  };
                  users.users.managarr = {
                    uid = lib.mkForce cfg.uid;
                    group = "managarr";
                    isSystemUser = true;
                  };
                  users.groups.managarr.gid = lib.mkForce cfg.gid;
                };
            };
        };
      };
  };
}
