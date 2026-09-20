# managarr — host configuration override for managarr container
#
# Points the reconcile at every service it reads: qBittorrent, the three *arrs
# and Bazarr for the library, autobrr and Prowlarr for the announce and search
# history that exists nowhere else, slskd for the Soulseek side, beets and
# cross-seed for the music library and the decision log, and the rip staging
# directory for self-rips.
#
# Every one of those is HTTP. Reading another service's SQLite over a bind
# mount was the alternative, and it is worse in every direction: it needs a
# matching uid, it couples this side to a schema the owner is free to change,
# and it puts a second process on a file its owner holds open in WAL mode.
#
# Secrets come in through sops as one env file — the *arr, Bazarr, autobrr and
# Prowlarr API keys — because a key on a command line is readable from the
# process table.
#
# The one mount is /mnt/media, read-only, to stat inode identity. managarr's
# own state lives in the postgresql container, not on disk here, so the
# container stays ephemeral.
{ den, ... }:
{
  den.aspects.hosts._.server._.containers._.managarr = {
    aspect-name = "hosts.server.containers.managarr";
    includes = [
      den.aspects.containers._.managarr
      den.aspects.infra._.clients._.container-managarr
      den.aspects.infra._.clients._.container-dns
      den.aspects.infra._.clients._.container-postgres
      den.aspects.infra._.clients._.container-qbittorrent
      den.aspects.infra._.clients._.container-sonarr
      den.aspects.infra._.clients._.container-radarr
      den.aspects.infra._.clients._.container-lidarr
      den.aspects.infra._.clients._.container-slskd
      den.aspects.infra._.clients._.container-bazarr
      den.aspects.infra._.clients._.container-autobrr
      den.aspects.infra._.clients._.container-prowlarr
      den.aspects.infra._.clients._.container-cross-seed
      den.aspects.infra._.clients._.container-beets
      den.aspects.containers._.postgresql
      den.aspects.infra._.networks._.media
      den.aspects.hosts._.server._.security
      den.aspects.hosts._.server._.networking._.vlans._.media
      den.aspects.infra._.clients._.container-loki
      den.aspects.infra._.clients._.container-prometheus
      den.aspects.infra._.clients._.container-tempo
    ];

    nixos =
      { config, ... }:
      {
        containerConfigs.managarr = {
          lldpd.enable = true;
          hostBridge = "br-media";
          macAddress = config.infra.clients.container-managarr.mac;
          localAddress = "${config.infra.clients.container-managarr.ip}/${toString config.infra.vlans.media.subnetPrefix}";
          gateway = config.infra.vlans.media.staticAddresses.gateway;
          port = config.infra.clients.container-managarr.ports.default;
          metricsPort = config.infra.clients.container-managarr.ports.metrics;
          dbHost = config.infra.clients.container-postgres.ip;
          dbPort = config.infra.clients.container-postgres.ports.default;
          qbittorrentUrl = "http://${config.infra.clients.container-qbittorrent.ip}:${toString config.infra.clients.container-qbittorrent.ports.webui}";
          sonarrUrl = "http://${config.infra.clients.container-sonarr.ip}:${toString config.infra.clients.container-sonarr.ports.default}";
          radarrUrl = "http://${config.infra.clients.container-radarr.ip}:${toString config.infra.clients.container-radarr.ports.default}";
          lidarrUrl = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.default}";
          # The sidecars, each inside its own *arr's container on the shared
          # 9910. They serve the same history their service does, with the
          # since-cursor the service does not offer — without one every hourly
          # pass replays the whole history to find what is new.
          sonarrSidecarUrl = "http://${config.infra.clients.container-sonarr.ip}:${toString config.infra.clients.container-sonarr.ports.api}";
          radarrSidecarUrl = "http://${config.infra.clients.container-radarr.ip}:${toString config.infra.clients.container-radarr.ports.api}";
          lidarrSidecarUrl = "http://${config.infra.clients.container-lidarr.ip}:${toString config.infra.clients.container-lidarr.ports.api}";
          # slskd is on the download VLAN, not media — reached by routing, the
          # same way qBittorrent is.
          slskdUrl = "http://${config.infra.clients.container-slskd.ip}:${toString config.infra.clients.container-slskd.ports.web}";
          bazarrUrl = "http://${config.infra.clients.container-bazarr.ip}:${toString config.infra.clients.container-bazarr.ports.default}";
          autobrrUrl = "http://${config.infra.clients.container-autobrr.ip}:${toString config.infra.clients.container-autobrr.ports.default}";
          prowlarrUrl = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.default}";
          # Both over HTTP rather than by reading their databases. beets serves
          # its library through its own web plugin; cross-seed gets
          # managarr-crossseed, a read-only sidecar running inside its container
          # beside the file, because its own API has no decisions route at all.
          crossSeedUrl = "http://${config.infra.clients.container-cross-seed.ip}:${toString config.infra.clients.container-cross-seed.ports.api}";
          beetsUrl = "http://${config.infra.clients.container-beets.ip}:${toString config.infra.clients.container-beets.ports.default}";
          # The remaining sidecars. Bazarr has none by design: its pass
          # decorates a full subtitle snapshot with history every run, so a
          # cursor would leave every subtitle first seen afterwards without
          # a provider or a score.
          autobrrSidecarUrl = "http://${config.infra.clients.container-autobrr.ip}:${toString config.infra.clients.container-autobrr.ports.api}";
          prowlarrSidecarUrl = "http://${config.infra.clients.container-prowlarr.ip}:${toString config.infra.clients.container-prowlarr.ports.api}";
          slskdSidecarUrl = "http://${config.infra.clients.container-slskd.ip}:${toString config.infra.clients.container-slskd.ports.api}";
          beetsSidecarUrl = "http://${config.infra.clients.container-beets.ip}:${toString config.infra.clients.container-beets.ports.api}";
          # Staging, not the library: the whipper log does not follow the audio
          # when beets hands it to Lidarr.
          ripsDir = "/mnt/media/complete/rips";
          envFile = config.sops.templates."managarr-env".path;
          # Hourly. A pass stats every file of every torrent across ~7000
          # torrents on the NFS mount, and the questions it answers — what the
          # client holds, what shares an inode — do not move faster than that.
          reconcileInterval = "hourly";
          # Every six hours. A dead torrent is not getting deader, but it holds
          # a client slot the whole time it sits there, and the tracker's
          # verdict is already final when the reconcile first sees it.
          #
          # Applying: a torrent the tracker has unregistered can never be
          # announced again, so seeding it moves no bytes and holds a slot
          # forever. The data goes with it — an *arr import and a cross-seed
          # inject are hardlinks, so unlinking the client's copy leaves theirs
          # and the bytes only go when nothing else has a link.
          #
          # Matching is on the tracker's MESSAGE, not the error code: code 5 is
          # generic and covers a peer-limit notice from a tracker that is
          # working perfectly. Anything under it that is not recognised as a
          # deletion is logged and left alone.
          cleanupInterval = "*-*-* 00/6:00:00";
          cleanupApply = true;
          # Daily, just after the cleanup it reads from. Deletes Sonarr and
          # Radarr entries no torrent provides any more — the file the cleanup
          # left behind when it removed the last torrent that carried it.
          #
          # Whether anything still provides it is answered TWICE, because once
          # was not enough. First from cross-seed's record of which infohashes
          # carry the same content, walked two hops. Then from the inode: the
          # reconcile stats every torrent file and every library file, so a
          # match says a live torrent is seeding these exact bytes.
          #
          # The second test exists because the first is knowably incomplete —
          # 70% of cross-seed offers carry no source infohash, and relying on
          # the graph alone proposed deleting five files while twelve live
          # torrents were seeding them.
          #
          # APPLYING. Verified against live qBittorrent rather than the
          # snapshot: no candidate is held by any live torrent by inode, by
          # name, or by content path.
          #
          # Deletion is permanent for content no longer on any tracker — that
          # is what makes a file a candidate in the first place. The episodes
          # stay monitored, so the *arr will search, but a release gone from
          # every indexer does not come back.
          orphanInterval = "daily";
          orphanApply = true;
          # The catch-all sweep: data on disk that no torrent owns.
          #
          # Categories are the torrent-managed ones only. soulseek, rips and
          # youtube share the same tree and were never torrents, so "no torrent
          # claims this" is not a statement about them at all.
          #
          # incompleteDir is deliberately unset: /var/tmp/qbittorrent belongs to
          # the qBittorrent container and is not mounted here, so the sweep
          # reports that check as skipped rather than as clean.
          # Hourly tag projection, replacing whatever each torrent carries —
          # including cross-seed's own tag, which managarr replaces with
          # xseed::inject derived from the link directory.
          #
          # No dry run for this one. The whole set is recomputed and replaced
          # every hour, so a wrong tag survives at most an hour and costs
          # nothing while it does; withholding it behind a flag would only mean
          # the tags stayed stale, which is the state this was built to end.
          tagInterval = "hourly";
          # cross-seed reads its own tag back — preFilter checks for it to
          # decide what NOT to search from — and 6.13.7 offers no way to turn
          # tagging off. Kept rather than replaced, so stripping it does not
          # make cross-seed search from its own injects.
          preserveTags = [ "cross-seed" ];
          # What each tracker demands before a torrent may be stopped. Either
          # limb clears it — ratio OR time — and the strictest user class is
          # always assumed, since being wrong that way only costs disk.
          #
          # Only the trackers whose rules have actually been read are listed.
          # The rest tag seed::unknown, which is an honest refusal to answer
          # rather than a guess that could cost an account.
          seedRequirements = {
            # TorrentLeech: "Seed a torrent to at least 1:1 ratio" OR "seed for
            # at least the minimum time required for your class". Registered is
            # the harshest class at 10 days, so that is the number used.
            # Seeding time counts only on fully completed torrents — a partial
            # one must reach 1:1.
            #
            # Both announce hosts are the same site and carry the same
            # obligation, so both are listed: the requirement is keyed on the
            # HOST, unlike the tracker tag which collapses them.
            "tracker.torrentleech.org" = {
              days = 10;
              ratio = 1.0;
            };
            "tracker.tleechreload.org" = {
              days = 10;
              ratio = 1.0;
            };

            # MyAnonaMouse: "you must seed for 72 hours within 30 days. The 72
            # hours starts accumulating AFTER download completion."
            #
            # NO RATIO LIMB — the rule names only time, so ratio cannot clear
            # it here however high it climbs. Time is accumulated rather than
            # continuous: "members are not obliged to seed back continuously".
            #
            # The 30-day deadline is NOT modelled. This answers "is the
            # obligation discharged", and a torrent at 40 hours is undischarged
            # whether it has 1 day left or 29. What is lost is urgency, not
            # correctness — see the seedRequirements option.
            "t.myanonamouse.net" = {
              days = 3;
              ratio = 0;
              timeNeedsComplete = true;
            };

            # SeedPool: "10-day seed time is required for all releases,
            # freeleech or not" with "torrents where you have downloaded less
            # than 10% are exempt".
            #
            # No ratio limb stated, and nothing about partials needing to
            # complete — the exemption IS its partial rule, and it runs the
            # opposite way to TorrentLeech's.
            "seedpool.org" = {
              days = 10;
              ratio = 0;
              timeNeedsComplete = false;
              exemptBelowProgress = 0.10;
            };

            # DigitalCore: "a torrent that you have started to download 10% or
            # more ... be seeded back for five days ... or until the ratio is
            # 1:1".
            #
            # timeNeedsComplete is false because DC settles it outright:
            # "seeding means making a torrent available for uploading, complete
            # or not".
            #
            # Not modelled: DC treats an hour without an announce as breaking
            # the obligation, and counts only time the tracker itself logged —
            # "the site is the only source of truth for seeding times, what
            # your client shows doesn't matter". qBittorrent's counter is
            # therefore an optimistic estimate here.
            "tracker.digitalcore.club" = {
              days = 5;
              ratio = 1.0;
              timeNeedsComplete = false;
              exemptBelowProgress = 0.10;
            };
            "trackerprxy.digitalcore.club" = {
              days = 5;
              ratio = 1.0;
              timeNeedsComplete = false;
              exemptBelowProgress = 0.10;
            };

            # RetroToon: "all torrents must be seeded for a total of 3 days (72
            # hours)" and "reaching a 1:1 ratio immediately clears a Hit & Run".
            # A total, so accumulated time is the right measure.
            "ann.retrotoon.world" = {
              days = 3;
              ratio = 1.0;
              timeNeedsComplete = false;
            };

            # DarkPeers: "seed all torrents to a ratio of 1:1 or 144 hours (6
            # days), whichever comes first".
            #
            # The only tracker so far carrying BOTH partial mechanics, and they
            # pull in opposite directions:
            #   3.2.1 enforced once "the user has downloaded at least 10%"
            #   3.2.2 "the seeding time will only start when 100% of the
            #         torrent is downloaded"
            #
            # So a torrent between 10% and 100% is charged but cannot discharge
            # by time — ratio is its only route. That falls out of the two
            # fields without a special case.
            #
            # Not modelled: 3.1's account-wide 0.8 ratio floor, which is a
            # property of the account rather than of any torrent.
            "darkpeers.org" = {
              days = 6;
              ratio = 1.0;
              timeNeedsComplete = true;
              exemptBelowProgress = 0.10;
            };

            # YU-Scene: "all torrents require a minimum of 5 days (120 hours)
            # Seedtime, REGARDLESS OF RATIO".
            #
            # ratio = 0 is the whole point here rather than an omission: this
            # tracker says outright that ratio does not discharge the
            # obligation, so a torrent at 10:1 is still held until the time is
            # served. Contrast DarkPeers and TorrentLeech, where either limb
            # works.
            #
            # "Seedtime starts only after 100% completion" and "a 15% download
            # buffer exists before HnR tracking begins" — note the buffer is
            # 15% here where DC, SeedPool and DarkPeers all use 10%.
            #
            # Not modelled: the 0.7 account ratio floor.
            "yu-scene.net" = {
              days = 5;
              ratio = 0;
              timeNeedsComplete = true;
              exemptBelowProgress = 0.15;
            };

            # BitPorn: "once you have downloaded more than 10% ... the seeding
            # requirements apply to that torrent REGARDLESS OF RATIO", and
            # "seedtime is counted only after the torrent reaches 100%
            # completion".
            #
            # TWO RULES DELIBERATELY NOT MODELLED, both resolved the strict way:
            #
            #   "seed for at least 7 days OR until there are at least 5 active
            #   seeders, whichever comes first" — the seeder limb could
            #   discharge this early, but it depends on a live swarm count that
            #   moves minute to minute. Acting on a stale one risks an HnR,
            #   while ignoring it only means seeding longer than strictly
            #   required, so the limb is dropped.
            #
            #   "completed requests are subject to an extended seedtime of 10
            #   days" — nothing here records which torrents filled a request,
            #   so 10 days is applied to all of them rather than 7 to some and
            #   10 to others chosen by guesswork.
            #
            # The result is a requirement at least as strict as the real one,
            # which is the side to err on.
            "bitporn.eu" = {
              days = 10;
              ratio = 0;
              timeNeedsComplete = true;
              exemptBelowProgress = 0.10;
            };

            # PreToMe: 72 hours, with no ratio limb and no partial rule
            # stated. Both announce hosts are the same site.
            #
            # timeNeedsComplete is true because nothing here says otherwise,
            # and of the two readings that is the one that cannot cause an
            # HnR — a partial simply stays obligated rather than banking time
            # it may not have earned.
            "stracker.pretome.info" = {
              days = 3;
              ratio = 0;
              timeNeedsComplete = true;
            };
            "tracker.preto.me" = {
              days = 3;
              ratio = 0;
              timeNeedsComplete = true;
            };
          };
          # Several sites announce under two hostnames, so the mapping collapses
          # them: without it a TorrentLeech torrent carries two tracker tags.
          trackerTags = {
            "tracker.torrentleech.org" = "TL";
            "tracker.tleechreload.org" = "TL";
            "tracker.digitalcore.club" = "DC";
            "trackerprxy.digitalcore.club" = "DC";
            "stracker.pretome.info" = "PTM";
            "tracker.preto.me" = "PTM";
            "seedpool.org" = "SP";
            "yu-scene.net" = "YS";
            "darkpeers.org" = "DP";
            "t.myanonamouse.net" = "MAM";
            "bitporn.eu" = "BP";
            "ann.retrotoon.world" = "RTW";
          };
          reapInterval = "daily";
          # Not applying yet — read one dry run first. It unlinks directory
          # trees, and the evidence for doing so is the absence of a claim.
          reapApply = false;
          completeDir = "/mnt/media/complete";
          linksDir = "/mnt/media/cross-seed/links";
          libraryDirs = [ "/mnt/media/library" ];
          reapCategories = [
            "tv"
            "movies"
            "books"
            "audiobooks"
            "comics"
            "xxx"
            # seasonpackarr's preImportPath sits in the same tree. Its packs are
            # ordinary torrents once assembled, so an abandoned one is stranded
            # data like any other — leaving the directory out meant nothing
            # would ever report it.
            "seasonpacks"
          ];
          # seasonpackarr assembles packs from episodes already in the client
          # and hardlinks them here. Torrents under it are a third origin, and
          # without this they look exactly like ordinary grabs.
          packsDir = "/mnt/media/complete/seasonpacks";
          # Hourly, alongside the tags — both read the same reconcile snapshot.
          #
          # Applying: the dry run resolved 222 of 259 pack files to the torrent
          # they were linked from. The 37 it cannot place are not failures of
          # the match — they are episodes the pack did not source from the
          # client. seasonpackarr logged "skipping hardlink" for the Reacher
          # ones, and the rest are combined episodes (S04E01E02) and EXTENDED
          # cuts that no single held torrent corresponds to.
          #
          # Writes associations only. Nothing here deletes, and a wrong row is
          # replaced by the next pass.
          packsInterval = "hourly";
          packsApply = true;
          bindMounts = {
            # Read-only on purpose. The reconcile stats download and library
            # paths to record inode identity and link counts; it has no reason
            # to write, and mounting it rw would make a bug in a reader capable
            # of touching media.
            "/mnt/media" = {
              hostPath = "/mnt/media";
              isReadOnly = true;
            };
          };
          nameservers = [
            config.infra.clients.container-dns.ip
            "1.1.1.1"
          ];
          logging = {
            enable = true;
            lokiAddress = "http://${config.infra.clients.container-loki.ip}:${toString config.infra.clients.container-loki.ports.default}";
            prometheusAddress = "http://${config.infra.clients.container-prometheus.ip}:${toString config.infra.clients.container-prometheus.ports.default}/api/v1/write";
            tempoAddress = "http://${config.infra.clients.container-tempo.ip}:${toString config.infra.clients.container-tempo.ports.otlp-http}/v1/traces";
            prometheusInputs = [ "managarr_metrics" ];
            extraSettings = {
              sources.managarr_metrics = {
                type = "prometheus_scrape";
                endpoints = [
                  "http://127.0.0.1:${toString config.infra.clients.container-managarr.ports.metrics}/metrics"
                ];
                scrape_interval_secs = 30;
              };
            };
          };
        };

        # The senders are containers, so managarr has to be listening before
        # they are. Same wave placement as the other media-VLAN services.
        # Postgres is a hard dependency: migrations run at startup.
        systemd.services."container@managarr" = {
          before = [ "container-wave-downstream.target" ];
          after = [
            "container-wave-apps.target"
            "container@postgresql.service"
          ];
          wants = [ "container@postgresql.service" ];
        };
      };
  };
}
