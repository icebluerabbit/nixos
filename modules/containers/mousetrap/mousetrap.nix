/*
  mousetrap — MyAnonaMouse grab authority (Go, single static binary).
  Serves /announce (autobrr webhook), /search, /torznab/{api,download} (indexer for
  the *arr ecosystem), /refresh/* + /shop (data collectors + shop policy) and /metrics.
  systemd timers (built from settings.refresh) POST to the /refresh|/shop endpoints on
  a schedule; all policy/params come from flags (settings below), so the timer bodies
  are bare. Binary is pkgs.homelab.mousetrap (see packages/go/mousetrap).
*/
_: {
  den.aspects.containers._.mousetrap = {
    aspect-name = "containers.mousetrap";
    includes = [ ];

    provides.to-hosts.nixos =
      {
        config,
        lib,
        pkgs,
        ...
      }:
      {
        options.containerConfigs.mousetrap =
          (import ../_options.nix {
            inherit lib;
            defaultPort = 8080;
            defaultUid = 5036;
            defaultGid = 5036;
          })
          // {
            settings = lib.mkOption {
              default = { };
              description = ''
                mousetrap service policy. Every threshold, list and toggle maps 1:1 to a
                binary flag and is passed on ExecStart; explicit even at its default value
                (declarative, no hidden runtime config). Domains mirror the binary: mam
                (source), client (delivery), accept (Layer-1), grab (Layer-2 policy),
                wedge, shop, torznab, webhook, plus refresh (timer cadence).
              '';
              type = lib.types.submodule {
                options = {
                  enable = lib.mkOption {
                    type = lib.types.bool;
                    default = false;
                    description = "Wire the mam seed secret and create the refresh timers";
                  };

                  mam = lib.mkOption {
                    default = { };
                    description = "MyAnonaMouse source client";
                    type = lib.types.submodule {
                      options = {
                        apiBase = lib.mkOption {
                          type = lib.types.str;
                          default = "https://www.myanonamouse.net";
                          description = "MAM API base URL";
                        };
                        seedSecretPath = lib.mkOption {
                          type = lib.types.str;
                          default = "/run/secrets/mam-api-id";
                          description = "In-container path to the mam_id seed cookie (sops, bind-mounted). Only wired when enable = true; otherwise MAM fails closed";
                        };
                        mediaTypeNames = lib.mkOption {
                          type = lib.types.attrsOf lib.types.str;
                          default = { };
                          description = ''
                            MAM search-API mediatype id → the name its IRC announce uses, so both
                            sources key policy on one vocabulary, e.g. { "2" = "Ebook"; }. MAM ships
                            ids we have no name for until one is announced; leave those out and they
                            stay their raw id — matching no policy list and showing up untranslated
                            in the metrics rather than being silently mislabeled.
                          '';
                        };
                      };
                    };
                  };

                  client = lib.mkOption {
                    default = { };
                    description = "Download-client delivery (qBittorrent today) for reactive grabs";
                    type = lib.types.submodule {
                      options = {
                        url = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "download client WebUI base URL (e.g. http://ip:port); mousetrap is auth-whitelisted";
                        };
                        categories = lib.mkOption {
                          type = lib.types.attrsOf lib.types.str;
                          default = { };
                          description = ''MAM media type → client category, e.g. { Ebook = "books"; Audiobook = "audiobooks"; }. An unlisted media type delivers uncategorized'';
                        };
                        addPaused = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                          description = "add torrents paused (manual review before seeding)";
                        };
                      };
                    };
                  };

                  accept = lib.mkOption {
                    default = { };
                    description = ''
                      Layer-1 acceptance criteria, evaluated per candidate on release fields only
                      (no account gates). Records mousetrap_would_accept_total{decision,reason}.
                      Empty list / 0 / "" = rule off.
                    '';
                    type = lib.types.submodule {
                      options = {
                        mediaTypes = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = ''allowed MAM media types, matched case-insensitively (e.g. [ "Ebook" "Audiobook" "Manga" ]); empty = any'';
                        };
                        categoriesAllow = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = ''MAM category allowlist (e.g. "Romance"); a release passes if it carries ANY listed category; empty = any'';
                        };
                        categoriesDeny = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = "MAM category denylist; a release is rejected if it carries ANY listed category (takes precedence over allow)";
                        };
                        filetypesAllow = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = "filetype allowlist (epub|m4b|mp3|pdf|cbz|cbr); a release passes if it announces ANY listed format; empty = any";
                        };
                        filetypesDeny = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = "filetype denylist; a release is rejected if it announces ANY listed format (takes precedence over allow)";
                        };
                        fictionTypes = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = ''MAM fiction flags to allow ("Fiction" / "Non-Fiction"); empty = any. MAM reuses category names on both sides of the line (Crime, Historical and Paranormal all appear as each), so this is the only rule that separates them. Releases carrying no flag — every torznab search result — always pass'';
                        };
                        languagesAllow = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = ''language substrings, matched case-insensitively (e.g. "english"); empty = any'';
                        };
                        sizeMinBytes = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "reject below this size in bytes; 0 = no floor";
                        };
                        sizeMaxBytes = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "reject above this size in bytes; 0 = no ceiling";
                        };
                        freeleechOnly = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                          description = "reject non-freeleech releases";
                        };
                        vipOnly = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                          description = "reject non-VIP releases (raw release vip flag; VIP-FL-for-us is Layer-2)";
                        };
                        excludeVip = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                          description = "reject VIP releases (raw release vip flag)";
                        };
                        nameIncludeRegex = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "require name/title to match this regex (case-insensitive); empty = off";
                        };
                        nameExcludeRegex = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "reject name/title matching this regex (case-insensitive); empty = off";
                        };
                      };
                    };
                  };

                  grab = lib.mkOption {
                    default = { };
                    description = ''
                      Layer-2 grab gate — account-state policy on Layer-1 survivors. Records
                      mousetrap_would_grab_total{decision,reason}. dryRun=true decides+logs and
                      adds nothing; flip to false to arm real grabs (delivery via client.*).
                    '';
                    type = lib.types.submodule {
                      options = {
                        unsatMargin = lib.mkOption {
                          type = lib.types.int;
                          default = 5;
                          description = "hold this many below unsat_limit (effective unsat = real + in-flight)";
                        };
                        inFlightWindowSec = lib.mkOption {
                          type = lib.types.int;
                          default = 1800;
                          description = "how long a grab counts as in-flight (covers MAM unsat.count + downloaded_bytes lag); expiry reconciles both";
                        };
                        bufferMinBytes = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "non-freeleech grabs require buffer_bytes ≥ this; 0 = off. FL (incl VIP-FL when we hold VIP) skips it";
                        };
                        dryRun = lib.mkOption {
                          type = lib.types.bool;
                          default = true;
                          description = "ARM SWITCH: true = decide + log only, add NOTHING; false = fetch .torrent + deliver to client";
                        };
                      };
                    };
                  };

                  wedge = lib.mkOption {
                    default = { };
                    description = ''
                      Personal Freeleech wedge strategy for non-FL grabs. A wedge (usetoken=FL at
                      download) makes the grab freeleech → no ratio/buffer cost, spends one wedge.
                      Inventory-gated with the same optimistic in-flight as unsat/buffer.
                    '';
                    type = lib.types.submodule {
                      options = {
                        enable = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                        };
                        reserve = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "never spend below this many wedges (effective = real − in-flight)";
                        };
                        proactiveMediaTypes = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = "B: MAM media types to always wedge when large (e.g. [ \"Audiobook\" ])";
                        };
                        proactiveMinSizeBytes = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "B: only proactively wedge grabs ≥ this size";
                        };
                        requireWedge = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                          description = "B: skip a proactive match when no wedge is spendable, instead of grabbing it at full ratio (small/non-proactive releases unaffected)";
                        };
                        rescueEnable = lib.mkOption {
                          type = lib.types.bool;
                          default = false;
                          description = "A: wedge a would-be buffer hold instead of holding";
                        };
                        rescueMinSizeBytes = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "A: only rescue grabs ≥ this size (don't waste wedges on tiny)";
                        };
                      };
                    };
                  };

                  shop = lib.mkOption {
                    default = { };
                    description = "bonusBuy policy (points → VIP / upload credit). dryRun logs intent only.";
                    type = lib.types.submodule {
                      options = {
                        dryRun = lib.mkOption {
                          type = lib.types.bool;
                          default = true;
                          description = "ARM SWITCH: true = decide + log only, buy NOTHING; false = call bonusBuy";
                        };
                        reserved = lib.mkOption {
                          type = lib.types.int;
                          default = 0;
                          description = "absolute bonus-point floor, never spent";
                        };
                        vip = lib.mkOption {
                          default = { };
                          type = lib.types.submodule {
                            options = {
                              enable = lib.mkOption {
                                type = lib.types.bool;
                                default = false;
                              };
                              cost = lib.mkOption {
                                type = lib.types.int;
                                default = 5000;
                                description = "bonus points per VIP purchase";
                              };
                              reserveMultiple = lib.mkOption {
                                type = lib.types.int;
                                default = 2;
                                description = "hold reserveMultiple × cost for renewals before buying upload";
                              };
                              buyableClasses = lib.mkOption {
                                type = lib.types.listOf lib.types.str;
                                default = [ ];
                                description = "class names permitted to buy VIP (Power User and above); empty = never (see mam_class_info for your class)";
                              };
                            };
                          };
                        };
                        upload = lib.mkOption {
                          default = { };
                          type = lib.types.submodule {
                            options = {
                              enable = lib.mkOption {
                                type = lib.types.bool;
                                default = false;
                              };
                              chunkGiB = lib.mkOption {
                                type = lib.types.int;
                                default = 50;
                              };
                              pointsPerGiB = lib.mkOption {
                                type = lib.types.int;
                                default = 500;
                              };
                            };
                          };
                        };
                      };
                    };
                  };

                  torznab = lib.mkOption {
                    default = { };
                    description = ''
                      Torznab indexer surface (/torznab/api + /torznab/download) for the *arr
                      ecosystem. search mints download links back to mousetrap so every grab still
                      runs the accept + grab policy; baseUrl is the absolute host those links point at.
                    '';
                    type = lib.types.submodule {
                      options = {
                        apiKey = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "require this apikey on torznab requests; empty = no auth (trusted LAN)";
                        };
                        baseUrl = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "absolute base URL for minted download links, e.g. http://mousetrap:8080";
                        };
                        linkTtlSec = lib.mkOption {
                          type = lib.types.int;
                          default = 3600;
                          description = "reject download links older than this many seconds; 0 = no expiry";
                        };
                      };
                    };
                  };

                  webhook = lib.mkOption {
                    default = { };
                    description = "Outbound notification on grab. Empty url = disabled.";
                    type = lib.types.submodule {
                      options = {
                        url = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "POST notifications to this URL; empty = disabled";
                        };
                        template = lib.mkOption {
                          type = lib.types.str;
                          default = "";
                          description = "Go text/template rendered as the POST body";
                        };
                        headers = lib.mkOption {
                          type = lib.types.attrsOf lib.types.str;
                          default = { };
                          description = "request headers, key = value";
                        };
                        events = lib.mkOption {
                          type = lib.types.listOf lib.types.str;
                          default = [ ];
                          description = "event types that fire: announce|grab|reject|hold|enqueue|bonus_received|spend; empty = none";
                        };
                      };
                    };
                  };

                  refresh = lib.mkOption {
                    default = { };
                    description = ''
                      Data-collector cadence. Each enabled entry gets one systemd timer + oneshot
                      that POSTs to the matching endpoint; search params live in the binary, so the
                      POST body is bare. Only created when settings.enable = true.
                    '';
                    type = lib.types.submodule {
                      options =
                        let
                          collector =
                            defaultInterval:
                            lib.mkOption {
                              default = { };
                              type = lib.types.submodule {
                                options = {
                                  enable = lib.mkOption {
                                    type = lib.types.bool;
                                    default = true;
                                  };
                                  intervalSec = lib.mkOption {
                                    type = lib.types.int;
                                    default = defaultInterval;
                                  };
                                };
                              };
                            };
                        in
                        {
                          account = collector 60;
                          freeleech = collector 1800;
                          bonus = collector 3600;
                          shop = collector 3600;
                        };
                    };
                  };
                };
              };
            };
          };

        config = {
          containers.mousetrap =
            let
              cfg = config.containerConfigs.mousetrap;
              # Resolved from host pkgs so the homelab overlay is in scope; the store
              # path pulls into the container closure via the ExecStart reference below.
              mousetrapBin = lib.getExe pkgs.homelab.mousetrap;
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
                  s = cfg.settings;
                  api = "http://127.0.0.1:${toString cfg.port}";

                  # flag helpers → GNU long-opts pflag understands. Bools always emit the
                  # explicit --flag=bool form (some Go defaults are true, e.g. *-dry-run).
                  f = {
                    str =
                      n: v:
                      lib.optionals (v != "") [
                        "--${n}"
                        v
                      ];
                    int = n: v: [
                      "--${n}"
                      (toString v)
                    ];
                    boolE = n: v: [ "--${n}=${lib.boolToString v}" ];
                    list =
                      n: vs:
                      lib.concatMap (v: [
                        "--${n}"
                        v
                      ]) vs;
                    kv =
                      n: a:
                      lib.concatLists (
                        lib.mapAttrsToList (k: v: [
                          "--${n}"
                          "${k}=${v}"
                        ]) a
                      );
                    durSec = n: v: [
                      "--${n}"
                      "${toString v}s"
                    ];
                  };

                  flags = [
                    "--listen"
                    "0.0.0.0"
                    "--port"
                    (toString cfg.port)
                    "--state-dir"
                    "/var/lib/mousetrap"
                  ]
                  # mam (source)
                  ++ f.str "mam-api-base" s.mam.apiBase
                  ++ f.str "mam-seed-path" (lib.optionalString s.enable s.mam.seedSecretPath)
                  ++ f.kv "mam-media-type-name" s.mam.mediaTypeNames
                  # client (delivery)
                  ++ f.str "client-url" s.client.url
                  ++ f.kv "client-category" s.client.categories
                  ++ f.boolE "client-paused" s.client.addPaused
                  # accept (Layer-1)
                  ++ f.list "accept-media-type" s.accept.mediaTypes
                  ++ f.list "accept-category-allow" s.accept.categoriesAllow
                  ++ f.list "accept-category-deny" s.accept.categoriesDeny
                  ++ f.list "accept-filetype-allow" s.accept.filetypesAllow
                  ++ f.list "accept-filetype-deny" s.accept.filetypesDeny
                  ++ f.list "accept-fiction-type" s.accept.fictionTypes
                  ++ f.list "accept-language" s.accept.languagesAllow
                  ++ f.int "accept-size-min" s.accept.sizeMinBytes
                  ++ f.int "accept-size-max" s.accept.sizeMaxBytes
                  ++ f.boolE "accept-freeleech-only" s.accept.freeleechOnly
                  ++ f.boolE "accept-vip-only" s.accept.vipOnly
                  ++ f.boolE "accept-exclude-vip" s.accept.excludeVip
                  ++ f.str "accept-name-include" s.accept.nameIncludeRegex
                  ++ f.str "accept-name-exclude" s.accept.nameExcludeRegex
                  # grab (Layer-2 policy)
                  ++ f.int "grab-unsat-margin" s.grab.unsatMargin
                  ++ f.durSec "grab-inflight-window" s.grab.inFlightWindowSec
                  ++ f.int "grab-buffer-min" s.grab.bufferMinBytes
                  ++ f.boolE "grab-dry-run" s.grab.dryRun
                  # wedge
                  ++ f.boolE "wedge-enable" s.wedge.enable
                  ++ f.int "wedge-reserve" s.wedge.reserve
                  ++ f.list "wedge-proactive-media-type" s.wedge.proactiveMediaTypes
                  ++ f.int "wedge-proactive-min-size" s.wedge.proactiveMinSizeBytes
                  ++ f.boolE "wedge-require" s.wedge.requireWedge
                  ++ f.boolE "wedge-rescue" s.wedge.rescueEnable
                  ++ f.int "wedge-rescue-min-size" s.wedge.rescueMinSizeBytes
                  # shop
                  ++ f.boolE "shop-dry-run" s.shop.dryRun
                  ++ f.int "shop-reserved" s.shop.reserved
                  ++ f.boolE "shop-vip-enable" s.shop.vip.enable
                  ++ f.int "shop-vip-cost" s.shop.vip.cost
                  ++ f.int "shop-vip-reserve-multiple" s.shop.vip.reserveMultiple
                  ++ f.list "shop-vip-buyable-class" s.shop.vip.buyableClasses
                  ++ f.boolE "shop-upload-enable" s.shop.upload.enable
                  ++ f.int "shop-upload-chunk-gib" s.shop.upload.chunkGiB
                  ++ f.int "shop-upload-points-per-gib" s.shop.upload.pointsPerGiB
                  # torznab
                  ++ f.str "torznab-apikey" s.torznab.apiKey
                  ++ f.str "torznab-base-url" s.torznab.baseUrl
                  ++ f.durSec "torznab-link-ttl" s.torznab.linkTtlSec
                  # webhook
                  ++ f.str "webhook-url" s.webhook.url
                  ++ f.str "webhook-template" s.webhook.template
                  ++ f.kv "webhook-header" s.webhook.headers
                  ++ f.list "webhook-event" s.webhook.events;

                  # collectors → one timer + oneshot each; the oneshot curls the endpoint
                  # (bare body — search params live in the binary flags above).
                  refreshers = lib.filter (r: r.enable) [
                    {
                      name = "account";
                      path = "/refresh/account";
                      inherit (s.refresh.account) enable;
                      interval = s.refresh.account.intervalSec;
                    }
                    {
                      name = "freeleech";
                      path = "/refresh/freeleech";
                      inherit (s.refresh.freeleech) enable;
                      interval = s.refresh.freeleech.intervalSec;
                    }
                    {
                      name = "bonus";
                      path = "/refresh/bonus";
                      inherit (s.refresh.bonus) enable;
                      interval = s.refresh.bonus.intervalSec;
                    }
                    {
                      name = "shop";
                      path = "/shop";
                      inherit (s.refresh.shop) enable;
                      interval = s.refresh.shop.intervalSec;
                    }
                  ];
                  refreshService = r: {
                    name = "mousetrap-refresh-${r.name}";
                    value = {
                      description = "mousetrap MAM refresh: ${r.name}";
                      after = [ "mousetrap.service" ];
                      path = [ pkgs.curl ];
                      serviceConfig.Type = "oneshot";
                      script = "curl -sS -m 30 -X POST ${api}${r.path} >/dev/null";
                    };
                  };
                  refreshTimer = r: {
                    name = "mousetrap-refresh-${r.name}";
                    value = {
                      wantedBy = [ "timers.target" ];
                      timerConfig = {
                        OnBootSec = "30s";
                        OnUnitActiveSec = "${toString r.interval}s";
                        Persistent = false;
                      };
                    };
                  };
                in
                {
                  imports = [ (import ../_base.nix { inherit lib cfg; }) ];

                  users.users.mousetrap = {
                    uid = lib.mkForce cfg.uid;
                    group = "mousetrap";
                    isSystemUser = true;
                    home = "/var/lib/mousetrap";
                  };
                  users.groups.mousetrap.gid = lib.mkForce cfg.gid;

                  systemd.services = {
                    mousetrap = {
                      description = "MyAnonaMouse grab authority (announce + torznab + refresh + shop + /metrics)";
                      wantedBy = [ "multi-user.target" ];
                      after = [ "network.target" ];
                      serviceConfig = {
                        ExecStart = "${mousetrapBin} ${lib.escapeShellArgs flags}";
                        User = "mousetrap";
                        Group = "mousetrap";
                        Restart = "on-failure";
                        RestartSec = "5s";
                      };
                    };
                  }
                  // lib.optionalAttrs s.enable (lib.listToAttrs (map refreshService refreshers));

                  systemd.timers = lib.optionalAttrs s.enable (lib.listToAttrs (map refreshTimer refreshers));
                };
            };
        };
      };
  };
}
