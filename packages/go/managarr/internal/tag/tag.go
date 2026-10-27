// Package tag projects managarr's view of the stack onto qBittorrent's tags.
//
// managarr is the sole authority: it computes the complete tag set for every
// torrent and REPLACES what is there. Nothing merges, so nothing accumulates.
//
// That is a deliberate reversal of how this client got into its current state.
// Three tools wrote tags — qbit-manage, cleanuparr and cross-seed — and two of
// them were removed. Their tags stayed exactly as they were on the day they
// stopped running, so 43% of torrents carry none at all and the rest carry a
// frozen snapshot. Two of those tags, noHL and cleanuparr-unlinked, were
// derived from hardlink counts, which is the specific defect managarr exists to
// correct: a hardlink count cannot tell an *arr import from an extracted
// archive, and it was wrong for everything cross-seed had linked.
//
// The vocabulary is partitioned with "::" so a partition can be filtered as a
// group in the client — tracker::, library::, state::, xseed::.
//
// WHAT EACH TAG IS WORTH is not uniform, and the difference is recorded here
// rather than left for someone to discover:
//
//	tracker::      exact. The announce hosts a torrent reported, mapped through
//	               configuration because one tracker announces under several
//	               hostnames and the domains do not resemble each other.
//	library::      exact. The *arr's own import record names the file it
//	               produced; this is the fact hardlink counting was guessing at.
//	state::        exact. The same signal the cleanup acts on.
//	xseed::inject  exact, and structural: cross-seed hardlinks what it injects
//	               into its link directory, so content living there IS an
//	               inject.
//	xseed::source  PARTIAL. See sourceOfInject.
package tag

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"homelab/clients/qbittorrent"
	"homelab/managarr/internal/model"
	"homelab/managarr/internal/query"
)

// Partition separators and the vocabulary built on them.
const (
	sep = "::"

	partTracker = "tracker"
	partLibrary = "library"
	partState   = "state"
	partXSeed   = "xseed"
	partPack    = "pack"
	partSeed    = "seed"

	valLinked       = "linked"
	valUnlinked     = "unlinked"
	valQueued       = "queued"
	valBlocked      = "blocked"
	valUnregistered = "unregistered"
	valInject       = "inject"
	valSource       = "source"
	valAssembled    = "assembled"
	valFeeds        = "feeds"
	valHnR          = "hnr"
	valFree         = "free"
	valUnknown      = "unknown"
)

// SeedRequirement is what one tracker demands before a torrent may stop.
//
// Satisfied by EITHER limb, not both: TorrentLeech's wording is "seed to at
// least 1:1 ratio OR seed for at least the minimum time for your class", and
// the others read the same way. Requiring both would hold torrents forever for
// no reason.
//
// ALWAYS CONFIGURE THE STRICTEST TIER. Where a tracker scales the requirement
// by user class — TorrentLeech runs 10 days for Registered down to none for
// VIP — the number here should be the harshest, because being wrong in that
// direction costs nothing and being wrong the other way costs the account.
type SeedRequirement struct {
	// Days is the minimum seeding time. Zero means the tracker sets none and
	// only Ratio can clear it.
	Days float64 `json:"days"`
	// Ratio is the minimum share ratio. Zero means there is no ratio path and
	// only time can clear it.
	Ratio float64 `json:"ratio"`
	// TimeNeedsComplete says seeding time only accrues once the torrent is
	// fully downloaded, which is how TorrentLeech states it: "partially
	// downloaded torrents will never accrue seeding time and need to be seeded
	// 1:1".
	//
	// NOT universal. SeedPool says nothing of the kind and instead exempts
	// small partials outright, so the two trackers treat a half-downloaded
	// torrent in opposite ways: TorrentLeech makes it harder to discharge,
	// SeedPool makes it free. Set from what each tracker actually says.
	TimeNeedsComplete bool `json:"timeNeedsComplete"`
	// ExemptBelowProgress waives the obligation entirely for a torrent barely
	// downloaded. SeedPool: "torrents where you have downloaded less than 10%
	// are exempt", so 0.10 there.
	//
	// Zero means no exemption, which is the safe default: a tracker that has
	// not said it forgives small partials has not forgiven them.
	ExemptBelowProgress float64 `json:"exemptBelowProgress"`
}

// met reports whether a torrent has discharged this requirement.
func (r SeedRequirement) met(ratio, seedSeconds, progress float64) bool {
	// Exemption first. Some trackers never raise an obligation for a torrent
	// barely downloaded — DigitalCore counts a download as "10% or more", and
	// SeedPool exempts below the same figure — so such a torrent is not
	// discharged so much as never charged.
	if r.ExemptBelowProgress > 0 && progress < r.ExemptBelowProgress {
		return true
	}
	if r.Ratio > 0 && ratio >= r.Ratio {
		return true
	}
	if r.Days > 0 {
		if r.TimeNeedsComplete && progress < 1.0 {
			// Time does not count for this torrent and never will until it
			// completes. Only the ratio limb remains, and it did not match.
			return false
		}
		// Accumulated, not continuous. Every tracker read so far measures a
		// total rather than an unbroken run — MyAnonaMouse is explicit that
		// "members are not obliged to seed back continuously" and that the
		// count resumes where it left off — which is exactly what
		// seeding_time_seconds reports.
		return seedSeconds >= r.Days*86400
	}
	// No time requirement configured: the ratio limb was the only one, and it
	// did not match.
	return false
}

// Setter is the piece of qbittorrent.Client this needs.
type Setter interface {
	SetTags(ctx context.Context, hashes, tags []string) error
	// Tags is the client's tag REGISTRY, which outlives the torrents.
	Tags(ctx context.Context) ([]string, error)
	// DeleteTags unregisters tags nothing carries.
	DeleteTags(ctx context.Context, tags []string) error
}

// Config is what the host declares.
type Config struct {
	// ClientName scopes every read, so a second torrent client's rows are not
	// projected onto this one's torrents.
	ClientName string
	// TrackerTags maps an announce host to the tag value it contributes.
	//
	// Required rather than derived. One tracker announces under several
	// hostnames — tracker.torrentleech.org and tracker.tleechreload.org are the
	// same site, as are tracker.digitalcore.club and trackerprxy.digitalcore.club
	// — and the domains share no common part to derive from. Without the map a
	// torrent would carry two tracker tags for one tracker.
	//
	// An unmapped host contributes its own hostname, so a new tracker shows up
	// as an ugly tag rather than as no tag at all.
	TrackerTags map[string]string
	// LinksDir is cross-seed's link directory. Content living under it was
	// injected by cross-seed, which is what makes xseed::inject exact.
	LinksDir string
	// PacksDir is seasonpackarr's preImportPath. Content living under it is a
	// season pack seasonpackarr ASSEMBLED from episodes already in the client,
	// rather than one that was downloaded.
	//
	// A third origin, and without it those torrents are indistinguishable from
	// ordinary grabs: they carry a tracker and a library state like anything
	// else, and nothing else in the model says where they came from.
	PacksDir string
	// SeedRequirements is what each tracker demands, by announce host.
	//
	// Declared rather than derived, for the same reason TrackerTags is: the
	// obligation is a policy the tracker publishes in prose and changes at
	// will, and no amount of observing torrents reveals it. A host with no
	// entry produces seed::unknown — an honest refusal to answer, which is
	// worth more than a guess that costs a ratio.
	SeedRequirements map[string]SeedRequirement
	// PreserveTags are tags another tool OWNS and READS, which this pass keeps
	// on any torrent already carrying one.
	//
	// The single exception to replacing tags wholesale, and it exists because
	// cross-seed is not merely decorating. preFilter.js decides what to search
	// FROM, and one of its tests is:
	//
	//	if (searchee.tags?.includes(TORRENT_TAG))
	//
	// TORRENT_TAG is the hardcoded string "cross-seed" — cross-seed 6.13.7 has
	// no setting that disables tagging, every branch of getTagsForNewTorrent
	// returns it, and createTag re-registers it on startup. Strip it and
	// cross-seed stops recognising its own injects, then searches from them:
	// wasted indexer budget against excludeRecentSearch, and cross-seeds of
	// cross-seeds.
	//
	// Preserved rather than emitted, so the tag stays cross-seed's to say. The
	// cost is that a torrent whose tag was already lost does not regain one
	// here; only cross-seed re-applying it on a future inject restores it.
	PreserveTags []string
}

// Tagger computes and applies tags.
type Tagger struct {
	q      *query.Query
	client Setter
	logger *slog.Logger
	cfg    Config
}

// New builds a tagger.
func New(db *gorm.DB, c Setter, logger *slog.Logger, cfg Config) *Tagger {
	return &Tagger{q: query.Use(db), client: c, logger: logger, cfg: cfg}
}

// NewFromClient is the production constructor.
func NewFromClient(db *gorm.DB, c *qbittorrent.Client, logger *slog.Logger, cfg Config) *Tagger {
	return New(db, c, logger, cfg)
}

// Stats reports what a run did.
type Stats struct {
	// Torrents is how many were considered.
	Torrents int
	// Changed is how many had a tag set differing from what they carry.
	Changed int
	// Applied is how many were actually written. Zero on a dry run.
	Applied int
	// Calls is how many requests that took — one per distinct tag set, not one
	// per torrent.
	Calls int
	// ByTag counts the computed vocabulary, which is the quickest way to see a
	// mapping mistake: a tracker::host tag means an unmapped announce host.
	ByTag map[string]int
	// Pruned is tags unregistered from the client's tag list — the sidebar
	// entries a retired tool leaves behind, which removing the tag from every
	// torrent does not clear.
	Pruned []string `json:",omitempty"`
	DryRun bool
	Took   time.Duration
	// Sample shows a few torrents whose tags would change, so a dry run can be
	// read rather than merely counted.
	Sample []Change `json:",omitempty"`
}

// Change is one torrent's before and after.
type Change struct {
	Infohash string
	From     string
	To       string
}

// maxSample bounds the sample in a result.
const maxSample = 25

// Run computes every torrent's tags and, unless dryRun, applies them.
//
// Writes only what differs. The client holds ~7600 torrents and the tag set for
// almost all of them is identical between runs, so rewriting unconditionally
// would be 7600 pointless mutations an hour.
func (t *Tagger) Run(ctx context.Context, dryRun bool) (Stats, error) {
	st := Stats{DryRun: dryRun, ByTag: map[string]int{}}
	started := time.Now()

	desired, current, err := t.compute(ctx)
	if err != nil {
		return st, err
	}
	st.Torrents = len(desired)

	// Grouped by the tag set they should carry, so one request covers every
	// torrent that wants the same tags.
	byTags := map[string][]string{}
	for hash, tags := range desired {
		for _, tg := range tags {
			st.ByTag[tg]++
		}
		want := strings.Join(tags, ",")
		if current[hash] == want {
			continue
		}
		st.Changed++
		if len(st.Sample) < maxSample {
			st.Sample = append(st.Sample, Change{
				Infohash: hash,
				From:     current[hash],
				To:       want,
			})
		}
		byTags[want] = append(byTags[want], hash)
	}

	if dryRun {
		st.Took = time.Since(started)
		return st, nil
	}

	// Sorted so a run's request order is stable, which makes two runs
	// comparable in a log.
	for _, want := range slices.Sorted(maps.Keys(byTags)) {
		hashes := byTags[want]
		var tags []string
		if want != "" {
			tags = strings.Split(want, ",")
		}
		if err := t.client.SetTags(ctx, hashes, tags); err != nil {
			return st, err
		}
		st.Calls++
		st.Applied += len(hashes)
	}

	// Then tidy the registry. Removing a tag from every torrent does NOT
	// unregister it — qBittorrent keeps it in the tag list, so it stays in the
	// sidebar with a count of zero. Seventeen tags from two retired tools sat
	// there looking current after every torrent had already been re-tagged.
	pruned, err := t.pruneRegistry(ctx, desired)
	if err != nil {
		return st, err
	}
	st.Pruned = pruned

	st.Took = time.Since(started)
	return st, nil
}

// pruneRegistry unregisters tags no torrent carries any more.
//
// Runs only after the writes, never on a dry run, and deletes exactly what the
// pass did not just assign. Since every torrent it manages was given its
// computed set, anything else registered is carried by nothing it manages.
//
// THE EDGE, stated because deleteTags removes a tag from torrents as well as
// from the list: a torrent added to the client since the last reconcile is not
// in this pass's view, so a tag only it carries counts as unused and is taken
// off it. cross-seed injecting between passes is the case that occurs — its
// tag is stripped an hour early. That costs nothing here, because cross-seed
// identifies its own injects by link directory rather than by the tag, and the
// next pass tags the torrent properly once the reconcile has seen it.
//
// Anything applied by hand goes the same way, which is the deal already made by
// replacing tags wholesale rather than merging.
func (t *Tagger) pruneRegistry(ctx context.Context, desired map[string][]string) ([]string, error) {
	registered, err := t.client.Tags(ctx)
	if err != nil {
		return nil, err
	}

	// Everything now in use, which after the writes above is what was computed.
	inUse := make(map[string]struct{}, len(registered))
	for _, tags := range desired {
		for _, tg := range tags {
			inUse[tg] = struct{}{}
		}
	}

	var dead []string
	for _, tg := range registered {
		if _, used := inUse[tg]; !used {
			dead = append(dead, tg)
		}
	}
	if len(dead) == 0 {
		return nil, nil
	}

	sort.Strings(dead)
	if err := t.client.DeleteTags(ctx, dead); err != nil {
		return nil, err
	}
	t.logger.InfoContext(ctx, "unregistered unused tags", "tags", dead)
	return dead, nil
}

// compute returns the tag set every torrent should carry, and what it carries.
func (t *Tagger) compute(ctx context.Context) (desired map[string][]string, current map[string]string, err error) {
	ct := t.q.ClientTorrent
	torrents, err := ct.WithContext(ctx).
		Select(ct.TorrentInfohash, ct.Tags, ct.ContentPath,
			ct.Ratio, ct.SeedingTimeSeconds, ct.Progress).
		Where(ct.ClientName.Eq(t.cfg.ClientName), ct.RemovedAt.IsNull()).
		Find()
	if err != nil {
		return nil, nil, fmt.Errorf("tag: torrents: %w", err)
	}

	trackers, err := t.trackerTags(ctx)
	if err != nil {
		return nil, nil, err
	}
	unregistered, err := t.unregistered(ctx)
	if err != nil {
		return nil, nil, err
	}
	linked, err := t.linked(ctx)
	if err != nil {
		return nil, nil, err
	}
	sources, err := t.sourceOfInject(ctx)
	if err != nil {
		return nil, nil, err
	}
	packFeeders, err := t.packSources(ctx)
	if err != nil {
		return nil, nil, err
	}
	queues, err := t.queueStates(ctx)
	if err != nil {
		return nil, nil, err
	}
	hosts, err := t.trackerHosts(ctx)
	if err != nil {
		return nil, nil, err
	}

	desired = make(map[string][]string, len(torrents))
	current = make(map[string]string, len(torrents))

	for _, row := range torrents {
		h := row.TorrentInfohash
		var carried []string
		if row.Tags != nil {
			// Normalized the same way the computed set is, so a difference in
			// spacing or order is not mistaken for a difference in content.
			current[h] = normalize(*row.Tags)
			carried = strings.Split(current[h], ",")
		}

		tags := slices.Clone(trackers[h])

		// Another tool's tag, kept because that tool reads it back. See
		// Config.PreserveTags.
		for _, keep := range t.cfg.PreserveTags {
			if slices.Contains(carried, keep) {
				tags = append(tags, keep)
			}
		}

		// One tag from this partition, most-settled first. A torrent that
		// reached the library is linked whatever its queue still says; only
		// when nothing was imported does the queue decide.
		//
		// unlinked is LAST and means what it says: no library file and no
		// pending work. Before the queue was consulted it also covered
		// everything downloaded a minute ago, which reads as "nothing will
		// happen to this" when an import had simply not been tried yet.
		switch {
		case has(linked, h):
			tags = append(tags, partLibrary+sep+valLinked)
		case queues[h] == queueBlocked:
			tags = append(tags, partLibrary+sep+valBlocked)
		case queues[h] == queueInFlight:
			tags = append(tags, partLibrary+sep+valQueued)
		default:
			tags = append(tags, partLibrary+sep+valUnlinked)
		}

		if _, ok := unregistered[h]; ok {
			tags = append(tags, partState+sep+valUnregistered)
		}

		// Structural rather than inferred: each tool hardlinks what it produces
		// into its own directory, so content living there came from it.
		if under(row.ContentPath, t.cfg.LinksDir) {
			tags = append(tags, partXSeed+sep+valInject)
		}
		if under(row.ContentPath, t.cfg.PacksDir) {
			tags = append(tags, partPack+sep+valAssembled)
		}
		if _, ok := packFeeders[h]; ok {
			tags = append(tags, partPack+sep+valFeeds)
		}

		// Whether this torrent may be stopped without breaching a tracker.
		// Only ever computed from what the trackers were declared to demand —
		// see Config.SeedRequirements.
		if len(t.cfg.SeedRequirements) > 0 {
			tags = append(tags, t.seedState(
				hosts[h], deref(row.Ratio), float64(derefInt(row.SeedingTimeSeconds)), deref(row.Progress)))
		}
		if _, ok := sources[h]; ok {
			tags = append(tags, partXSeed+sep+valSource)
		}

		sort.Strings(tags)
		desired[h] = slices.Compact(tags)
	}
	return desired, current, nil
}

// deref reads an optional float, treating absent as zero.
//
// Zero is the safe reading for every use here: an unmeasured ratio must not
// clear a requirement, and an unmeasured progress must not pass for complete.
func deref(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

// derefInt reads an optional count, treating absent as zero — see deref.
func derefInt(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// under reports whether a content path sits inside a tool's output directory.
//
// The separator is appended deliberately: without it a sibling directory whose
// name merely starts with the root would match, which is the same mistake that
// made a hand-rolled version of the reap sweep report live torrents as orphans.
func under(contentPath *string, root string) bool {
	if root == "" || contentPath == nil {
		return false
	}
	return strings.HasPrefix(*contentPath, strings.TrimSuffix(root, "/")+"/")
}

// normalize renders a tag string the way the computed set is rendered.
//
// qBittorrent returns tags comma-separated with a space after each comma and in
// its own order. Comparing that against a computed set directly reports every
// torrent as changed, forever.
func normalize(s string) string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return strings.Join(slices.Compact(out), ",")
}

// seedState decides a torrent's standing against every tracker it announces to.
//
// WORST CASE WINS. A torrent on three trackers carries three separate
// obligations, and stopping it breaches whichever are outstanding — so one
// unmet requirement makes the whole torrent seed::hnr regardless of how
// thoroughly the others are satisfied.
//
// A torrent whose trackers are ALL unconfigured is seed::unknown. Mixed is not
// unknown: if one tracker has a stated requirement, that requirement is a fact
// about the torrent and is reported, because an answer about a real obligation
// beats silence about it.
func (t *Tagger) seedState(hosts []string, ratio, seedSeconds, progress float64) string {
	var known bool
	for _, host := range hosts {
		req, ok := t.cfg.SeedRequirements[host]
		if !ok {
			continue
		}
		known = true
		if !req.met(ratio, seedSeconds, progress) {
			return partSeed + sep + valHnR
		}
	}
	if !known {
		return partSeed + sep + valUnknown
	}
	return partSeed + sep + valFree
}

// trackerHosts maps each torrent to the announce hosts it reported.
//
// Kept apart from trackerTags because the seed requirement is keyed on the
// HOST, while the tag is keyed on the mapped code — two hosts collapsing to one
// tracker tag still carry their own obligations.
func (t *Tagger) trackerHosts(ctx context.Context) (map[string][]string, error) {
	tt := t.q.TorrentTracker
	rows, err := tt.WithContext(ctx).
		Select(tt.TorrentInfohash, tt.TrackerAnnounceHost).
		Find()
	if err != nil {
		return nil, fmt.Errorf("tag: tracker hosts: %w", err)
	}

	out := map[string][]string{}
	for _, r := range rows {
		if !slices.Contains(out[r.TorrentInfohash], r.TrackerAnnounceHost) {
			out[r.TorrentInfohash] = append(out[r.TorrentInfohash], r.TrackerAnnounceHost)
		}
	}
	return out, nil
}

// trackerTags maps each torrent to the tracker tags it should carry.
func (t *Tagger) trackerTags(ctx context.Context) (map[string][]string, error) {
	tt := t.q.TorrentTracker
	rows, err := tt.WithContext(ctx).
		Select(tt.TorrentInfohash, tt.TrackerAnnounceHost).
		Find()
	if err != nil {
		return nil, fmt.Errorf("tag: trackers: %w", err)
	}

	out := map[string][]string{}
	for _, r := range rows {
		host := r.TrackerAnnounceHost
		value, ok := t.cfg.TrackerTags[host]
		if !ok {
			// Unmapped: tag with the host itself. Visible and ugly, which is
			// the point — a missing mapping should be obvious in the client
			// rather than silently producing an untagged torrent.
			value = host
		}
		tag := partTracker + sep + value
		if !slices.Contains(out[r.TorrentInfohash], tag) {
			out[r.TorrentInfohash] = append(out[r.TorrentInfohash], tag)
		}
	}
	return out, nil
}

// unregistered is the set whose tracker has dropped the record.
func (t *Tagger) unregistered(ctx context.Context) (map[string]struct{}, error) {
	tt := t.q.TorrentTracker
	rows, err := tt.WithContext(ctx).
		Select(tt.TorrentInfohash, tt.Message).
		Where(tt.StatusCode.Eq(qbittorrent.TrackerError), tt.Message.IsNotNull()).
		Find()
	if err != nil {
		return nil, fmt.Errorf("tag: unregistered: %w", err)
	}

	out := map[string]struct{}{}
	for _, r := range rows {
		// Matching on the MESSAGE, not the status code: code 5 is a generic
		// tracker error covering things that are not deletions at all,
		// including a peer-limit notice from a tracker working perfectly.
		if r.Message != nil && qbittorrent.IsUnregistered(*r.Message) {
			out[r.TorrentInfohash] = struct{}{}
		}
	}
	return out, nil
}

// linked is the set an *arr has imported into a library.
//
// From the import record rather than from a link count. That distinction is the
// reason this service exists: a hardlink count is wrong for anything extracted
// from an archive and wrong again for anything cross-seed linked, while the
// import record names the file outright.
func (t *Tagger) linked(ctx context.Context) (map[string]struct{}, error) {
	lf := t.q.LibraryFile
	var hashes []string
	err := lf.WithContext(ctx).
		Distinct(lf.TorrentInfohash).
		Where(lf.TorrentInfohash.IsNotNull(), lf.RemovedAt.IsNull()).
		Pluck(lf.TorrentInfohash, &hashes)
	if err != nil {
		return nil, fmt.Errorf("tag: linked: %w", err)
	}

	out := make(map[string]struct{}, len(hashes))
	for _, h := range hashes {
		out[h] = struct{}{}
	}
	return out, nil
}

// queueState is a torrent's standing in an *arr's queue.
//
// The ZERO VALUE means no *arr is working on it, which is what a lookup for an
// unqueued torrent yields. Ordered by severity so the worse of two services'
// views wins by a plain comparison.
type queueState int

const (
	// queueInFlight: downloading, or waiting for an import that will happen.
	queueInFlight queueState = iota + 1
	// queueBlocked: the *arr has it and will NOT import it without help.
	queueBlocked
)

// Queue states, as the *arr's trackedDownloadState spells them.
//
// These are the STATE half of the composite trackedStatus the reconcile
// records — "warning/importBlocked" — so the state is taken alone rather than
// matching the pair, which varies with the status half independently.
const (
	stateImportBlocked = "importBlocked"
	stateImportFailed  = "importFailed"
)

// queueStates maps each queued torrent to how stuck it is.
//
// The distinction the library:: partition turns on. A download completed a
// minute ago and one the *arr has refused to import for weeks are both absent
// from the library, and calling both "unlinked" says nothing useful about
// either — the first needs no attention and the second needs nothing but.
//
// Where two *arrs both queue one torrent, the WORSE state wins. A blocked
// import is a fact about the torrent; another service getting on with its own
// copy does not make it untrue.
func (t *Tagger) queueStates(ctx context.Context) (map[string]queueState, error) {
	qi := t.q.QueueItem
	rows, err := qi.WithContext(ctx).
		Select(qi.TorrentInfohash, qi.TrackedStatus).
		Where(qi.TorrentInfohash.IsNotNull()).
		Find()
	if err != nil {
		return nil, fmt.Errorf("tag: queue states: %w", err)
	}

	out := make(map[string]queueState, len(rows))
	for _, r := range rows {
		if r.TorrentInfohash == nil {
			continue
		}
		state := queueInFlight
		if r.TrackedStatus != nil {
			switch trackedState(*r.TrackedStatus) {
			case stateImportBlocked, stateImportFailed:
				state = queueBlocked
			}
		}
		h := *r.TorrentInfohash
		if state > out[h] {
			out[h] = state
		}
	}
	return out, nil
}

// trackedState takes the state half of a recorded "status/state" pair.
func trackedState(trackedStatus string) string {
	if i := strings.LastIndex(trackedStatus, "/"); i >= 0 {
		return trackedStatus[i+1:]
	}
	return trackedStatus
}

// has reports membership, so the tag switch reads as a sequence of conditions
// rather than alternating between comma-ok and comparison.
func has(set map[string]struct{}, key string) bool {
	_, ok := set[key]
	return ok
}

// packSources is the set an assembled season pack was built from.
//
// The tag that matters most for deletion in this vocabulary. A pack and the
// episodes it was assembled from are THE SAME BYTES under two names — the pack
// is hardlinks to them — so removing an episode torrent's data while the pack
// still seeds breaks the pack, and the reverse breaks the episodes. Neither is
// visible from the client, where both look like ordinary torrents.
//
// Read from pack_sources, which the packs pass writes. Empty until that pass is
// applying, so the tag appears only once the associations are real rather than
// proposed — which is the right order: a tag claiming a dependency that was
// never recorded is worse than no tag.
func (t *Tagger) packSources(ctx context.Context) (map[string]struct{}, error) {
	ps := t.q.PackSource
	var hashes []string
	err := ps.WithContext(ctx).
		Distinct(ps.SourceInfohash).
		Pluck(ps.SourceInfohash, &hashes)
	if err != nil {
		return nil, fmt.Errorf("tag: pack sources: %w", err)
	}

	out := make(map[string]struct{}, len(hashes))
	for _, h := range hashes {
		out[h] = struct{}{}
	}
	return out, nil
}

// sourceOfInject is the set cross-seed searched FROM to produce an inject.
//
// PARTIAL COVERAGE, and knowingly so. Only the webhook ever states both ends of
// a cross-seed pairing — cross-seed's own database identifies a searchee by
// NAME, so recovering the pairing from it means matching names, which is not
// something this codebase does. Coverage therefore begins at the point the
// webhook started recording both infohashes and grows from there; at the time
// of writing 494 of 3204 injects have a recorded source.
//
// The consequence to keep in mind: ABSENCE OF THIS TAG IS NOT EVIDENCE. It
// means no pairing was recorded, not that the torrent has no cross-seeds. It
// cannot be backfilled, and it improves on its own as cross-seed runs, because
// the whole set is recomputed every pass.
//
// Deliberately restricted to sources of REAL injects rather than of any
// candidate offer. cross-seed proposes candidates it then rejects, and it
// matches torrents you already hold — so "was named as a source somewhere"
// marks 1514 torrents where only 328 produced anything.
func (t *Tagger) sourceOfInject(ctx context.Context) (map[string]struct{}, error) {
	if t.cfg.LinksDir == "" {
		return nil, nil
	}

	// The injects, identified structurally.
	ct := t.q.ClientTorrent
	var injects []string
	err := ct.WithContext(ctx).
		Distinct(ct.TorrentInfohash).
		Where(
			ct.ClientName.Eq(t.cfg.ClientName),
			ct.RemovedAt.IsNull(),
			ct.ContentPath.Like(strings.TrimSuffix(t.cfg.LinksDir, "/")+"/%"),
		).
		Pluck(ct.TorrentInfohash, &injects)
	if err != nil {
		return nil, fmt.Errorf("tag: injects: %w", err)
	}
	if len(injects) == 0 {
		return nil, nil
	}

	o := t.q.Offer
	var sources []string
	err = o.WithContext(ctx).
		Distinct(o.SourceInfohash).
		Where(
			o.Kind.Eq(model.OfferCrossSeedCandidate),
			o.SourceInfohash.IsNotNull(),
			o.Infohash.In(injects...),
		).
		Pluck(o.SourceInfohash, &sources)
	if err != nil {
		return nil, fmt.Errorf("tag: inject sources: %w", err)
	}

	out := make(map[string]struct{}, len(sources))
	for _, h := range sources {
		out[h] = struct{}{}
	}
	return out, nil
}
