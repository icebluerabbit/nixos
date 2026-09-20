// Command managarr records the state of the download and library stack in
// PostgreSQL, so that questions about provenance can be answered by a query
// rather than by inference.
//
// It replaces cleanuparr and qbit-manage. Neither could express the actual
// policy — keep seeding until the content leaves the library or the tracker's
// hit-and-run window closes — because both inferred library membership from
// hardlink counts, which is only correct when the *arr imported by hardlink.
// An *arr import record states the mapping outright: it carries the download id
// alongside the file it produced.
//
// Two mechanisms, in that order of authority:
//
//   - Reconcile passes poll each service's API and upsert a full snapshot.
//     These are authoritative, because a webhook missed during a restart is
//     gone and anything the model must be right about has to survive that.
//   - Webhook ingest records what the APIs forget: a torrent added and removed
//     between two passes, a cross-seed injection, a slskd failure reason that
//     expires with the transfer, and the infohash autobrr emits nowhere else.
//
// Nothing here writes to the stack. It reads, and it records.
package main

// The typed query API in internal/query is generated from store.Models.
//
// Declared at the module root because go generate runs each directive with the
// containing file's directory as the working directory, and the generator
// writes to a path relative to that.
//
//go:generate go run ./_gen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/pflag"

	"homelab/app"
	"homelab/clients/arr"
	"homelab/clients/autobrr"
	"homelab/clients/bazarr"
	"homelab/clients/beets"
	"homelab/clients/prowlarr"
	"homelab/clients/qbittorrent"
	"homelab/clients/slskd"
	"homelab/httpx"
	"homelab/managarr/internal/cleanup"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/ingest"
	"homelab/managarr/internal/orphan"
	"homelab/managarr/internal/packs"
	"homelab/managarr/internal/reap"
	"homelab/managarr/internal/reconcile"
	"homelab/managarr/internal/server"
	"homelab/managarr/internal/stats"
	"homelab/managarr/internal/store"
	"homelab/managarr/internal/tag"
	"homelab/sidecar"
)

// Service instance names.
//
// The model keys every observation on the INSTANCE that made it, not on the
// software, because two Sonarrs are two sources of truth whose ids share no
// namespace. This homelab runs one of each, so the instance name is the
// software name — but it is named once here rather than spelled out at each
// call site, because a reconcile pass and a webhook handler disagreeing about
// it would silently produce two Service rows for one service.
const (
	nameSonarr        = "sonarr"
	nameRadarr        = "radarr"
	nameLidarr        = "lidarr"
	nameProwlarr      = "prowlarr"
	nameAutobrr       = "autobrr"
	nameBazarr        = "bazarr"
	nameCrossSeed     = "cross-seed"
	nameSeasonpackarr = "seasonpackarr"
	nameUnpackerr     = "unpackerr"
	nameQBittorrent   = "qbittorrent"
	nameSlskd         = "slskd"
)

// newApp wires the command line to the work. Split out of main only so
// main_test can drive it: the flag names and their mapping onto Config's keys
// are the one thing here that can be wrong, and a typo in either half would be
// silent until the unit ran.
func newApp(cfg *server.Config) app.App {
	return app.App{
		Name:  "managarr",
		Short: "Record the state of the download and library stack",
		Long: "Polls each configured service's API into PostgreSQL and " +
			"serves one webhook endpoint per source under /hook/ for the " +
			"events those APIs do not retain. Snapshots are authoritative; " +
			"webhooks are enrichment. Every hook answers 200 so a sender is " +
			"never disabled by a parser fault on this side.",
		Config: cfg,
		Flags: func(f *pflag.FlagSet) {
			f.String("addr", cfg.Addr, "listen address for the hook endpoints")
			f.Duration("shutdown-timeout", cfg.ShutdownTimeout, "grace period for in-flight requests")
			f.String("db-dsn", cfg.DBDSN, "PostgreSQL DSN; the schema is reconciled at startup")
			f.String("qbittorrent-url", cfg.QBittorrentURL, "qBittorrent WebUI base URL; empty skips it")
			f.String("sonarr-url", cfg.SonarrURL, "Sonarr base URL; empty skips it")
			f.String("radarr-url", cfg.RadarrURL, "Radarr base URL; empty skips it")
			f.String("lidarr-url", cfg.LidarrURL, "Lidarr base URL; empty skips it")
			f.String("sonarr-sidecar-url", cfg.SonarrSidecarURL,
				"managarr-sonarr base URL; empty reads Sonarr's history directly")
			f.String("radarr-sidecar-url", cfg.RadarrSidecarURL,
				"managarr-radarr base URL; empty reads Radarr's history directly")
			f.String("lidarr-sidecar-url", cfg.LidarrSidecarURL,
				"managarr-lidarr base URL; empty reads Lidarr's history directly")
			f.String("bazarr-url", cfg.BazarrURL, "Bazarr base URL; empty skips it")
			f.String("slskd-url", cfg.SlskdURL, "slskd base URL; empty skips it")
			f.String("autobrr-url", cfg.AutobrrURL, "autobrr base URL; empty skips it")
			f.Bool("autobrr-full", cfg.AutobrrFull, "walk autobrr's whole release history instead of stopping at the first page with nothing new")
			f.String("prowlarr-url", cfg.ProwlarrURL, "Prowlarr base URL; empty skips it")
			f.Bool("prowlarr-full", cfg.ProwlarrFull, "walk Prowlarr's whole history instead of stopping at the first page with nothing new")
			f.String("cross-seed-url", cfg.CrossSeedURL, "crossseed-api sidecar base URL; empty skips the decision replay")
			f.String("beets-url", cfg.BeetsURL, "beets web plugin base URL; empty skips the tag pass")
			f.String("autobrr-sidecar-url", cfg.AutobrrSidecarURL,
				"managarr-autobrr base URL; empty reads autobrr's releases directly")
			f.String("prowlarr-sidecar-url", cfg.ProwlarrSidecarURL,
				"managarr-prowlarr base URL; empty reads Prowlarr's history directly")
			f.String("slskd-sidecar-url", cfg.SlskdSidecarURL,
				"managarr-slskd base URL; empty reads slskd's event log directly")
			f.String("beets-sidecar-url", cfg.BeetsSidecarURL,
				"managarr-beets base URL; empty reads beets directly")
			f.Duration("stats-interval", cfg.StatsInterval,
				"how often the domain gauges are recomputed from the database")
			f.String("rips-dir", cfg.RipsDir, "CD-rip staging root, laid out as <artist>/<album>; empty skips the scan")
			f.String("complete-dir", cfg.CompleteDir, "download tree as <category>/<entry>; empty skips the reap sweep")
			f.String("links-dir", cfg.LinksDir, "cross-seed link tree as <tracker>/<entry>; empty skips that check")
			f.String("incomplete-dir", cfg.IncompleteDir, "partial-download root; empty skips that check")
			f.String("packs-dir", cfg.PacksDir, "seasonpackarr preImportPath; content under it is an assembled pack")
			f.StringSlice("library-dirs", cfg.LibraryDirs, "library roots, used to bound the missing-file check")
			f.StringSlice("reap-categories", cfg.ReapCategories,
				"download categories torrents manage; everything else in the tree is left alone")
			f.StringToString("tracker-tags", cfg.TrackerTags,
				"announce host to tracker:: tag value; unmapped hosts tag with the hostname")
			f.StringSlice("preserve-tags", cfg.PreserveTags,
				"tags another tool owns and reads back; kept rather than replaced")
			f.String("seed-requirements", cfg.SeedRequirementsJSON,
				`per-tracker seed obligations as JSON, keyed by announce host: `+
					`{"host":{"days":10,"ratio":1.0,"timeNeedsComplete":true}}`)
		},
		Run: func(ctx context.Context, env *app.Env) error {
			// Fail to start rather than serve without a database: a recorder
			// that answers 200 and drops everything is worse than one that is
			// visibly down.
			db, err := store.OpenGorm(ctx, cfg.DBDSN, env.Logger)
			if err != nil {
				return err
			}
			if err := store.AutoMigrate(db, env.Logger); err != nil {
				return err
			}

			d := dao.New(db, env.Logger)
			lookups := dao.NewLookups(d)

			// The reconcile is driven by a systemd timer hitting
			// POST /reconcile, not by a ticker in here: a full pass stats every
			// file of every torrent over NFS and has nothing to do with serving
			// a webhook inside a request timeout.
			passes, err := buildPasses(cfg, d, lookups, env)
			if err != nil {
				return err
			}

			// Domain gauges on their own ticker. Separate from the reconcile
			// deliberately: a pass is hourly because it stats every file over
			// NFS, and these are indexed aggregates costing milliseconds —
			// pinning them to the pass would throw away every trend finer than
			// an hour.
			//
			// Started before the listener so a dashboard opened immediately
			// after a restart sees the stack rather than an empty graph.
			collector := stats.New(db, env.Logger, env.Metrics)
			go collector.Run(ctx, cfg.StatsInterval)

			ing := ingest.New(d, lookups, env.Logger, ingest.Config{
				Sonarr:        nameSonarr,
				Radarr:        nameRadarr,
				Lidarr:        nameLidarr,
				Prowlarr:      nameProwlarr,
				Autobrr:       nameAutobrr,
				CrossSeed:     nameCrossSeed,
				Seasonpackarr: nameSeasonpackarr,
				Unpackerr:     nameUnpackerr,
				QBittorrent:   nameQBittorrent,
				Slskd:         nameSlskd,
			})
			deps := server.Deps{
				Logger: env.Logger,
				Ingest: ing.Handle,
			}
			if len(passes) == 0 {
				env.Logger.Warn("no sources configured; POST /reconcile disabled")
			} else {
				deps.Reconcile = func(ctx context.Context) error {
					// Every pass runs, whatever the ones before it did.
					//
					// Stopping at the first error meant one pass could silently
					// take down the others: a duplicate-key fault in sonarr's
					// catalog step aborted the loop before radarr and lidarr
					// were reached, and the library ingest for all three stayed
					// frozen for nineteen hours while the timer kept reporting
					// a failure that named only sonarr.
					//
					// The passes are independent by construction — each polls
					// one service and upserts its own rows — so there is no
					// ordering requirement that a failure could violate.
					var failed []error
					for _, p := range passes {
						if err := p(ctx); err != nil {
							failed = append(failed, err)
						}
					}
					return errors.Join(failed...)
				}
			}

			// Cleanup needs the torrent client, because it is the only thing
			// here that writes to one. Without a qBittorrent URL there is
			// nothing to delete from and the endpoint stays off.
			if cfg.QBittorrentURL != "" {
				qb, err := qbittorrent.New(httpx.Config{
					BaseURL: cfg.QBittorrentURL,
					Logger:  env.Logger,
				})
				if err != nil {
					return fmt.Errorf("qbittorrent client: %w", err)
				}
				cleaner := cleanup.New(db, qb, env.Logger, nameQBittorrent)
				deps.Cleanup = func(ctx context.Context, dryRun bool) (any, error) {
					return cleaner.Run(ctx, dryRun)
				}

				// Same client: tagging is the other thing managarr writes to
				// qBittorrent, and it needs the link directory to tell an
				// inject from an original.
				// Parsed at startup rather than per pass: a malformed value
				// should stop the service, not silently tag every torrent
				// seed::unknown once an hour forever.
				seedReqs := map[string]tag.SeedRequirement{}
				if cfg.SeedRequirementsJSON != "" {
					if err := json.Unmarshal([]byte(cfg.SeedRequirementsJSON), &seedReqs); err != nil {
						return fmt.Errorf("seed-requirements: %w", err)
					}
				}

				tagger := tag.NewFromClient(db, qb, env.Logger, tag.Config{
					ClientName:       nameQBittorrent,
					TrackerTags:      cfg.TrackerTags,
					LinksDir:         cfg.LinksDir,
					PacksDir:         cfg.PacksDir,
					PreserveTags:     cfg.PreserveTags,
					SeedRequirements: seedReqs,
				})
				deps.Tags = func(ctx context.Context, dryRun bool) (any, error) {
					return tagger.Run(ctx, dryRun)
				}
			}

			// Pack sources need no client at all: the association is between
			// what is on disk under the pack directory and what the database
			// says the client holds.
			if cfg.PacksDir != "" {
				resolver := packs.New(db, d, env.Logger, packs.Config{
					ClientName: nameQBittorrent,
					PacksDir:   cfg.PacksDir,
				})
				deps.Packs = func(ctx context.Context, dryRun bool) (any, error) {
					return resolver.Run(ctx, dryRun)
				}
			}

			// Orphans needs the *arrs, because they are what holds the library
			// entry and the only thing that can remove one. Sonarr and Radarr
			// only — see the orphan package on why music is excluded.
			//
			// Built here rather than reused from the reconcile passes: those
			// are closures over their own clients by the time this runs, and
			// reaching into them to retrieve one would couple the two for no
			// gain. A second HTTP client is free.
			arrClients := map[string]*arr.Client{}
			for _, a := range []struct {
				kind   arr.Kind
				name   string
				url    string
				envKey string
			}{
				{arr.Sonarr, nameSonarr, cfg.SonarrURL, "SONARR_APIKEY"},
				{arr.Radarr, nameRadarr, cfg.RadarrURL, "RADARR_APIKEY"},
			} {
				if a.url == "" {
					continue
				}
				key := os.Getenv(a.envKey)
				if key == "" {
					// Already reported by buildPasses for the same *arr, which
					// runs first. Skipping rather than failing twice.
					continue
				}
				c, err := arr.New(a.kind, httpx.Config{BaseURL: a.url, Logger: env.Logger}, key)
				if err != nil {
					return fmt.Errorf("%s client for orphans: %w", a.kind, err)
				}
				arrClients[a.name] = c
			}
			if len(arrClients) > 0 {
				remover := orphan.NewFromArr(db, env.Logger, arrClients)
				deps.Orphans = func(ctx context.Context, dryRun bool) (any, error) {
					return remover.Run(ctx, dryRun)
				}
			}

			// The reap sweep needs no client at all — it compares what the
			// database says is held against what is on disk. Enabled by having
			// somewhere to look.
			if cfg.CompleteDir != "" || cfg.LinksDir != "" || cfg.IncompleteDir != "" {
				// The torrent client, where configured, is read directly for
				// the claimed set rather than through the reconcile snapshot —
				// see reap.heldPaths.
				var holder reap.Holder
				if cfg.QBittorrentURL != "" {
					qb, err := qbittorrent.New(httpx.Config{
						BaseURL: cfg.QBittorrentURL,
						Logger:  env.Logger,
					})
					if err != nil {
						return fmt.Errorf("qbittorrent client for reap: %w", err)
					}
					holder = qb
				}
				scanner := reap.New(db, holder, env.Logger, reap.Config{
					CompleteRoot:   cfg.CompleteDir,
					LinksRoot:      cfg.LinksDir,
					IncompleteRoot: cfg.IncompleteDir,
					LibraryRoots:   cfg.LibraryDirs,
					Categories:     cfg.ReapCategories,
				})
				deps.Reap = func(ctx context.Context, dryRun bool) (any, error) {
					return scanner.Run(ctx, dryRun)
				}
			}

			srv, err := server.New(deps, env.Metrics)
			if err != nil {
				return err
			}
			return server.Serve(ctx, cfg, srv.Handler(), env.Logger)
		},
	}
}

// pass is one reconcile step.
type pass func(context.Context) error

// buildPasses assembles the reconcile passes for whichever sources are
// configured.
//
// The ORDER IS THE DEPENDENCY ORDER and every step depends on the ones before
// it:
//
//	qbittorrent  torrents exist, so anything naming one can reference it
//	autobrr      announces and their verdicts
//	prowlarr     indexers and search history
//	*arr         the catalog — content, files, grabs, queue
//	cross-seed   candidates, which reference torrents
//	slskd        the Soulseek side
//	bazarr       subtitles, which reference *arr content and files
//	beets        tags, which reference Lidarr's content and files
//	rips         discs, which reference nothing and are read from disk
//	resolve      every cross-service link, once everything else is present
//
// API keys come from the environment rather than argv — a key on a command line
// is visible in the process table to anything that can read /proc.
func buildPasses(cfg *server.Config, d *dao.DAO, l *dao.Lookups, env *app.Env) ([]pass, error) {
	var passes []pass
	metrics := reconcile.NewMetrics(env.Metrics)

	if cfg.QBittorrentURL != "" {
		qb, err := qbittorrent.New(httpx.Config{
			BaseURL: cfg.QBittorrentURL,
			Logger:  env.Logger,
		})
		if err != nil {
			return nil, fmt.Errorf("qbittorrent client: %w", err)
		}
		rec := reconcile.NewQBittorrent(qb, d, l, env.Logger, nameQBittorrent)
		passes = append(passes, logged(env, metrics, "qbittorrent", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.AutobrrURL != "" {
		key := os.Getenv("AUTOBRR_APIKEY")
		if key == "" {
			// Configured but unusable. Skipping silently would leave every
			// announce and every rejection permanently empty with nothing in
			// the log to explain it.
			return nil, fmt.Errorf("autobrr-url is set but AUTOBRR_APIKEY is empty")
		}
		ab, err := autobrr.New(httpx.Config{
			BaseURL: cfg.AutobrrURL,
			Logger:  env.Logger,
			Headers: http.Header{"X-API-Token": []string{key}},
		})
		if err != nil {
			return nil, fmt.Errorf("autobrr client: %w", err)
		}
		// pageSize 1000: the corpus is six figures and each release carries
		// its actions inline, so a larger page is a multi-megabyte response.
		walk, err := sidecarWalk(env, d, cfg.AutobrrSidecarURL, nameAutobrr, 1000)
		if err != nil {
			return nil, err
		}
		rec := reconcile.NewAutobrr(ab, walk, d, l, env.Logger, nameAutobrr, 1000, cfg.AutobrrFull)
		passes = append(passes, logged(env, metrics, "autobrr", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.ProwlarrURL != "" {
		key := os.Getenv("PROWLARR_APIKEY")
		if key == "" {
			return nil, fmt.Errorf("prowlarr-url is set but PROWLARR_APIKEY is empty")
		}
		pw, err := prowlarr.New(httpx.Config{
			BaseURL: cfg.ProwlarrURL,
			Logger:  env.Logger,
			Headers: http.Header{"X-Api-Key": []string{key}},
		})
		if err != nil {
			return nil, fmt.Errorf("prowlarr client: %w", err)
		}
		walk, err := sidecarWalk(env, d, cfg.ProwlarrSidecarURL, nameProwlarr, 500)
		if err != nil {
			return nil, err
		}
		rec := reconcile.NewProwlarr(pw, walk, d, l, env.Logger, nameProwlarr, 500, cfg.ProwlarrFull)
		passes = append(passes, logged(env, metrics, "prowlarr", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	arrs := []struct {
		kind   arr.Kind
		name   string
		url    string
		envKey string
		// sidecarURL is managarr-<kind>, which serves the same history with
		// the since-cursor the *arr itself does not offer. Empty falls back to
		// reading /history directly, which works and re-reads everything.
		sidecarURL string
	}{
		{arr.Sonarr, nameSonarr, cfg.SonarrURL, "SONARR_APIKEY", cfg.SonarrSidecarURL},
		{arr.Radarr, nameRadarr, cfg.RadarrURL, "RADARR_APIKEY", cfg.RadarrSidecarURL},
		{arr.Lidarr, nameLidarr, cfg.LidarrURL, "LIDARR_APIKEY", cfg.LidarrSidecarURL},
	}
	for _, a := range arrs {
		if a.url == "" {
			continue
		}
		key := os.Getenv(a.envKey)
		if key == "" {
			return nil, fmt.Errorf("%s is set but %s is empty", a.kind, a.envKey)
		}
		c, err := arr.New(a.kind, httpx.Config{BaseURL: a.url, Logger: env.Logger}, key)
		if err != nil {
			return nil, fmt.Errorf("%s client: %w", a.kind, err)
		}

		var walk *reconcile.SidecarWalk
		if a.sidecarURL != "" {
			sc, err := sidecar.NewClient(httpx.Config{BaseURL: a.sidecarURL, Logger: env.Logger})
			if err != nil {
				return nil, fmt.Errorf("managarr-%s client: %w", a.kind, err)
			}
			walk = reconcile.NewSidecarWalk(sc, d, env.Logger, string(a.kind), 500)
		}

		rec := reconcile.NewArr(c, walk, d, l, env.Logger, a.name, 500)
		passes = append(passes, logged(env, metrics, a.name, func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.CrossSeedURL != "" {
		cs, err := sidecar.NewClient(httpx.Config{BaseURL: cfg.CrossSeedURL, Logger: env.Logger})
		if err != nil {
			return nil, fmt.Errorf("managarr-crossseed client: %w", err)
		}
		rec := reconcile.NewCrossSeed(cs, d, l, env.Logger, nameCrossSeed, 1000)
		passes = append(passes, logged(env, metrics, "cross-seed", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.SlskdURL != "" {
		// No credentials: slskd runs with authentication.disabled and is gated
		// at Caddy, so the API is open on its VLAN.
		sk, err := slskd.New(httpx.Config{BaseURL: cfg.SlskdURL, Logger: env.Logger})
		if err != nil {
			return nil, fmt.Errorf("slskd client: %w", err)
		}
		// 500 to match the sidecar's own cap: every download event embeds a
		// full transfer object, so a larger page is megabytes per request.
		walk, err := sidecarWalk(env, d, cfg.SlskdSidecarURL, nameSlskd, 500)
		if err != nil {
			return nil, err
		}
		rec := reconcile.NewSlskd(sk, walk, d, l, env.Logger, nameSlskd)
		passes = append(passes, logged(env, metrics, "slskd", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.BazarrURL != "" {
		key := os.Getenv("BAZARR_APIKEY")
		if key == "" {
			return nil, fmt.Errorf("bazarr-url is set but BAZARR_APIKEY is empty")
		}
		bz, err := bazarr.New(httpx.Config{BaseURL: cfg.BazarrURL, Logger: env.Logger}, key)
		if err != nil {
			return nil, fmt.Errorf("bazarr client: %w", err)
		}
		rec := reconcile.NewBazarr(bz, d, l, env.Logger, nameBazarr, nameSonarr, nameRadarr)
		passes = append(passes, logged(env, metrics, "bazarr", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.BeetsURL != "" {
		bt, err := beets.New(httpx.Config{BaseURL: cfg.BeetsURL, Logger: env.Logger})
		if err != nil {
			return nil, fmt.Errorf("beets client: %w", err)
		}
		walk, err := sidecarWalk(env, d, cfg.BeetsSidecarURL, "beets", 1000)
		if err != nil {
			return nil, err
		}
		rec := reconcile.NewBeets(bt, walk, d, l, env.Logger)
		passes = append(passes, logged(env, metrics, "beets", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if cfg.RipsDir != "" {
		rec := reconcile.NewRips(d, l, env.Logger, cfg.RipsDir)
		passes = append(passes, logged(env, metrics, "rips", func(ctx context.Context) (any, error) {
			return rec.Run(ctx)
		}))
	}

	if len(passes) == 0 {
		return nil, nil
	}

	// Always last, and unconditional: it is the only step that links one
	// service's rows to another's, and it has nothing to read until they have
	// all written.
	res := reconcile.NewResolve(d, env.Logger)
	passes = append(passes, logged(env, metrics, "resolve", func(ctx context.Context) (any, error) {
		return res.Run(ctx)
	}))

	return passes, nil
}

// logged wraps a pass so it reports its own counts.
//
// The stats structs differ per pass and are logged as one value rather than
// flattened into keys: slog renders the struct, and a per-pass field list here
// would be a second place to update every time a pass learns to count something
// new — which is exactly the kind of duplication that goes stale silently.
func logged(env *app.Env, m *reconcile.Metrics, name string, run func(context.Context) (any, error)) pass {
	// Registered when the pass is BUILT, not when it first runs. A *Vec
	// publishes nothing until a label value is used, so on a fresh process
	// none of these series exist — and an alert on "no successful pass in
	// three hours" cannot fire against a series that is absent. managarr
	// restarts on every deploy, which makes that window exactly the one after
	// a change.
	m.Register(name)

	return func(ctx context.Context) error {
		env.Logger.Info("reconcile starting", "pass", name)
		started := time.Now()
		done := m.Start(name)

		result, err := run(ctx)
		done(err)
		if err != nil {
			return fmt.Errorf("reconcile %s: %w", name, err)
		}
		env.Logger.Info("reconcile",
			"pass", name, "stats", result, "took", time.Since(started).String())
		return nil
	}
}

func main() {
	cfg := server.Defaults()
	newApp(&cfg).Main()
}

// sidecarWalk builds a cursored walk over one sidecar, or nil when none is
// configured.
//
// A nil walk is the documented fallback, not a failure: every pass that takes
// one can still read its service directly. That path is correct and merely
// wasteful, so a sidecar being absent costs the cursor rather than the pass.
func sidecarWalk(env *app.Env, d *dao.DAO, url, source string, pageSize int) (*reconcile.SidecarWalk, error) {
	if url == "" {
		return nil, nil
	}
	c, err := sidecar.NewClient(httpx.Config{BaseURL: url, Logger: env.Logger})
	if err != nil {
		return nil, fmt.Errorf("managarr-%s client: %w", source, err)
	}
	return reconcile.NewSidecarWalk(c, d, env.Logger, source, pageSize), nil
}
