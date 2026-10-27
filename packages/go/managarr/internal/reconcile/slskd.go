package reconcile

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"homelab/clients/slskd"
	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
	"homelab/sidecar"
	"homelab/sidecar/slskdsvc"
)

// Slskd reconciles the Soulseek side: search history, live transfers, and the
// search results behind the files that were actually taken.
//
// The three have very different durability. slskd keeps every search it has ever
// run — there is no retention setting for them, which is why this homelab runs a
// separate pruner — so the search history is effectively complete. Transfers age
// out on slskd's own schedule, so a poll only ever sees what is in flight.
//
// Results are kept only for files that were downloaded, and that is a
// deliberate cut rather than a shortcut. A Soulseek result is dead on arrival:
// the file existed only while the peer holding it was online, so a peer who
// offered a better copy in March is offline and unreachable now and the row
// answers a question nobody can act on. Recording everything would be 1.47
// million rows and several hundred megabytes to store a million things that can
// never be retrieved. A result behind a real download is the opposite — it says
// what quality was actually obtained and which query found it, neither of which
// the download record itself holds.
type Slskd struct {
	client *slskd.Client
	// walk reads the event log through the sidecar with a stored cursor.
	// Nil falls back to walking slskd directly from offset 0 every pass.
	walk       *SidecarWalk
	dao        *dao.DAO
	lookups    *dao.Lookups
	logger     *slog.Logger
	clientName string
}

// NewSlskd builds the pass.
func NewSlskd(c *slskd.Client, walk *SidecarWalk, d *dao.DAO, l *dao.Lookups, logger *slog.Logger, clientName string) *Slskd {
	return &Slskd{client: c, walk: walk, dao: d, lookups: l, logger: logger, clientName: clientName}
}

// SlskdStats reports what a run did.
type SlskdStats struct {
	Searches  int
	Detailed  int
	Results   int
	Downloads int
	Uploads   int
	Took      time.Duration
}

// Run performs one pass.
func (s *Slskd) Run(ctx context.Context) (SlskdStats, error) {
	var st SlskdStats
	started := time.Now()
	s.lookups.Reset()

	if _, err := s.lookups.Client(ctx, s.clientName, model.ClientSlskd); err != nil {
		return st, err
	}

	// Downloads first: they are what decides which results are worth keeping,
	// so the set has to be complete before any detail is examined.
	downloads, uploads, err := s.syncDownloads(ctx)
	if err != nil {
		return st, err
	}
	st.Downloads, st.Uploads = downloads, uploads

	searches, err := s.client.Searches(ctx)
	if err != nil {
		return st, err
	}
	ids, err := s.syncSearches(ctx, searches)
	if err != nil {
		return st, err
	}
	st.Searches = len(searches)

	st.Detailed, st.Results, err = s.syncResults(ctx, searches, ids)
	if err != nil {
		return st, err
	}

	st.Took = time.Since(started)
	return st, nil
}

// syncSearches records the search history and returns each search's id.
//
// Ids are minted here rather than read back, because the results written next
// need them and re-reading a thousand rows one at a time to learn what GORM
// assigned costs a query each.
func (s *Slskd) syncSearches(ctx context.Context, searches []slskd.Search) (map[string]model.ID, error) {
	known, err := s.knownSearches(ctx)
	if err != nil {
		return nil, err
	}

	rows := make([]model.SoulseekSearch, 0, len(searches))
	out := make(map[string]model.ID, len(searches))

	for _, sr := range searches {
		startedAt := searchTime(sr.StartedAt)
		key := searchKey(sr.SearchText, startedAt)

		id, ok := known[key]
		if !ok {
			id = model.NewID()
			known[key] = id
		}
		out[sr.ID] = id

		rows = append(rows, model.SoulseekSearch{
			Base:      model.Base{ID: id},
			Query:     sr.SearchText,
			StartedAt: startedAt,
			EndedAt:   timePtr(sr.EndedAt),
			FileCount: intPtr(sr.FileCount),
			PeerCount: intPtr(sr.ResponseCount),
			State:     strPtr(sr.State),
		})
	}

	if err := dao.Upsert(ctx, s.dao, rows,
		[]string{"query", "started_at"},
		[]string{"ended_at", "file_count", "peer_count", "state"},
	); err != nil {
		return nil, fmt.Errorf("slskd: searches: %w", err)
	}
	return out, nil
}

// syncResults reads each search's answers and keeps the ones that became
// downloads.
//
// Every search is read at most once, ever. A completed search's answers never
// change, slskd keeps every search indefinitely, and one detail response runs to
// a couple of hundred kilobytes — re-reading 1,300 of them hourly would move
// hundreds of megabytes to rediscover nothing.
//
// Incomplete searches are skipped rather than read partially: answers are still
// arriving, and marking one read would mean never returning to it.
//
// Known limitation: a download that begins after its search has been read is not
// matched retroactively. In practice a download follows its own search within
// minutes and both fall inside one pass, so the case this misses is a search
// read on one pass whose file is taken on a later one.
func (s *Slskd) syncResults(
	ctx context.Context,
	searches []slskd.Search,
	ids map[string]model.ID,
) (detailed, results int, err error) {
	downloaded, err := s.downloadedFiles(ctx)
	if err != nil {
		return 0, 0, err
	}
	pending, err := s.unreadSearches(ctx)
	if err != nil {
		return 0, 0, err
	}

	for _, sr := range searches {
		id, ok := ids[sr.ID]
		if !ok {
			continue
		}
		if _, unread := pending[id]; !unread {
			continue
		}
		if !sr.IsComplete {
			continue
		}
		// Nobody answered, so there is nothing to fetch — but it still counts
		// as read, or it would be reconsidered on every pass forever.
		if sr.ResponseCount > 0 {
			detail, err := s.client.SearchDetail(ctx, sr.ID)
			if err != nil {
				return detailed, results, err
			}
			detailed++

			n, err := s.writeResults(ctx, id, detail.Responses, downloaded)
			if err != nil {
				return detailed, results, err
			}
			results += n
		}

		if err := s.markRead(ctx, id); err != nil {
			return detailed, results, err
		}
	}
	return detailed, results, nil
}

// writeResults records the answers that became downloads.
//
// The match is exact rather than inferred: a result and a download are the same
// thing when peer and remote path agree, which is precisely the natural key of
// soulseek_downloads.
func (s *Slskd) writeResults(
	ctx context.Context,
	searchID model.ID,
	responses []slskd.Response,
	downloaded map[string]struct{},
) (int, error) {
	kept := filterResults(responses, downloaded)
	rows := make([]model.SoulseekResult, 0, len(kept))

	for _, k := range kept {
		// The peer row is an enforced foreign key. Resolved only for a file
		// that is being kept, so a search answered by two hundred peers does
		// not create two hundred peer rows for one match.
		peer, err := s.lookups.SoulseekPeer(ctx, k.peer)
		if err != nil {
			return 0, err
		}
		if peer == nil {
			continue
		}
		rows = append(rows, model.SoulseekResult{
			SearchID:     searchID,
			PeerUsername: *peer,
			RemotePath:   k.file.Filename,
			SizeBytes:    int64Ptr(k.file.Size),
			Extension:    strPtr(k.file.Extension),
			Bitrate:      intPtr(k.file.BitRate),
			BitDepth:     intPtr(k.file.BitDepth),
			SampleRate:   intPtr(k.file.SampleRate),
			LengthSec:    intPtr(k.file.Length),
			Locked:       boolPtr(k.file.IsLocked),
		})
	}

	// Insert rather than upsert: a result is what one peer said at one moment
	// in answer to one search. A repeat is slskd saying it twice, not new
	// information.
	if err := dao.Insert(ctx, s.dao, rows,
		[]string{"search_id", "peer_username", "remote_path"}); err != nil {
		return 0, fmt.Errorf("slskd: results for %s: %w", searchID, err)
	}
	return len(rows), nil
}

// markRead records that a search's answers have been examined.
func (s *Slskd) markRead(ctx context.Context, id model.ID) error {
	now := time.Now().UTC()
	ss := s.dao.Q().SoulseekSearch
	_, err := ss.WithContext(ctx).
		Where(ss.ID.Eq(id)).
		Update(ss.ResultsFetchedAt, now)
	if err != nil {
		return fmt.Errorf("slskd: mark search %s read: %w", id, err)
	}
	return nil
}

// downloadedFiles is every file this stack has ever pulled off Soulseek.
//
// Read from the table rather than from the live API: slskd expires a completed
// transfer, so its API only knows what is in flight, while the table accumulates
// everything past polls and webhooks recorded.
func (s *Slskd) downloadedFiles(ctx context.Context) (map[string]struct{}, error) {
	sd := s.dao.Q().SoulseekDownload
	rows, err := sd.WithContext(ctx).
		Select(sd.PeerUsername, sd.RemotePath).Find()
	if err != nil {
		return nil, fmt.Errorf("slskd: downloaded files: %w", err)
	}
	out := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		out[fileKey(r.PeerUsername, r.RemotePath)] = struct{}{}
	}
	return out, nil
}

// unreadSearches is the set whose answers have not been examined.
func (s *Slskd) unreadSearches(ctx context.Context) (map[model.ID]struct{}, error) {
	var ids []model.ID
	ss := s.dao.Q().SoulseekSearch
	err := ss.WithContext(ctx).
		Where(ss.ResultsFetchedAt.IsNull()).
		Pluck(ss.ID, &ids)
	if err != nil {
		return nil, fmt.Errorf("slskd: unread searches: %w", err)
	}
	out := make(map[model.ID]struct{}, len(ids))
	for _, id := range ids {
		out[id] = struct{}{}
	}
	return out, nil
}

// fileKey identifies one file on one peer, which is all the identity a Soulseek
// file has.
func fileKey(peer, path string) string { return peer + "\x00" + path }

// syncDownloads records every file slskd has pulled, past and present.
//
// Two sources, and the order matters. The event log is the authority: it is
// persisted, it reaches back to the day slskd started keeping it, and it is the
// only place the LOCAL path of a download is ever stated. The live transfer
// listing is read afterwards for the one thing the log lacks — transfers still
// running, which have not completed and so have emitted no completion event.
//
// Reading the listing alone, which is what this did before, meant the table
// held whatever happened to be in flight at the moment of a pass and nothing
// else: slskd expires a completed transfer, so its own API reported seven files
// against 30,809 in the log.
func (s *Slskd) syncDownloads(ctx context.Context) (downloadCount, resultCount int, err error) {
	count, uploads, err := s.syncEvents(ctx)
	if err != nil {
		return count, uploads, err
	}

	downloads, err := s.client.Downloads(ctx)
	if err != nil {
		return count, uploads, err
	}
	for _, d := range downloads {
		peer, err := s.lookups.SoulseekPeer(ctx, d.Username)
		if err != nil {
			return count, uploads, err
		}
		if peer == nil {
			continue
		}
		for _, grp := range d.Directory {
			for _, f := range grp.Files {
				if err := s.upsertDownload(ctx, *peer, f); err != nil {
					return count, uploads, err
				}
				count++
			}
		}
	}
	return count, uploads, nil
}

// upsertDownload records one transfer's current state.
//
// A finished transfer is never moved backwards. slskd expires a completed
// transfer from its API, so a later poll cannot see it, and the webhook may
// redeliver an older event — the guard is a WHERE rather than a conditional
// assignment so the rule reads as a rule.
func (s *Slskd) upsertDownload(ctx context.Context, peer string, f slskd.DownloadFile) error {
	row := model.SoulseekDownload{
		ClientName:   s.clientName,
		PeerUsername: peer,
		RemotePath:   f.Filename,
		State:        strPtr(f.State),
		Bytes:        int64Ptr(f.BytesTransferred),
		SizeBytes:    int64Ptr(f.Size),
		RequestedAt:  timePtr(f.RequestedAt.Time),
		StartedAt:    timePtr(f.StartedAt.Time),
		FinishedAt:   timePtr(f.EndedAt.Time),
	}

	if err := dao.Insert(ctx, s.dao, []model.SoulseekDownload{row},
		[]string{"peer_username", "remote_path"}); err != nil {
		return fmt.Errorf("slskd: download %q: %w", f.Filename, err)
	}

	sd := s.dao.Q().SoulseekDownload
	_, err := sd.WithContext(ctx).
		Where(
			sd.PeerUsername.Eq(peer),
			sd.RemotePath.Eq(f.Filename),
			sd.FinishedAt.IsNull(),
		).
		UpdateSimple(
			dao.Assign(sd.State, row.State),
			dao.Assign(sd.Bytes, row.Bytes),
			dao.Assign(sd.SizeBytes, row.SizeBytes),
			dao.Assign(sd.StartedAt, row.StartedAt),
			dao.Assign(sd.FinishedAt, row.FinishedAt),
		)
	return err
}

// knownSearches loads the ids already assigned, keyed as searchKey does.
func (s *Slskd) knownSearches(ctx context.Context) (map[string]model.ID, error) {
	ss := s.dao.Q().SoulseekSearch
	rows, err := ss.WithContext(ctx).
		Select(ss.ID, ss.Query, ss.StartedAt).Find()
	if err != nil {
		return nil, fmt.Errorf("slskd: known searches: %w", err)
	}
	out := make(map[string]model.ID, len(rows))
	for _, r := range rows {
		out[searchKey(r.Query, searchTime(r.StartedAt))] = r.ID
	}
	return out, nil
}

// searchTime normalises a search's start to microsecond precision.
//
// This is what makes the id map work. slskd sends seven fractional digits
// (100ns); PostgreSQL's timestamptz holds six and ROUNDS to fit. Handing it a
// value it has to round means the time that comes back out is not the one that
// went in, so a preloaded key would never match the incoming one — every search
// would look new, be minted a fresh id, lose the ON CONFLICT, and leave every
// result pointing at a search row that does not exist.
//
// Rounding here rather than truncating, to match what the database does.
func searchTime(t time.Time) time.Time {
	return t.UTC().Round(time.Microsecond)
}

// searchKey is the natural key of a search: what was asked, and when.
func searchKey(query string, startedAt time.Time) string {
	return query + "\x00" + startedAt.Format(time.RFC3339Nano)
}

// keptResult is one search answer that became a download.
type keptResult struct {
	peer string
	file slskd.ResponseFile
}

// filterResults reduces a search's answers to the files that were taken.
//
// Pure and separate from the write so the cut can be exercised without a
// database — it is the decision that keeps this table at dozens of rows instead
// of 1.47 million, and getting it wrong in either direction is silent.
//
// A file with no peer or no path is dropped rather than kept with a blank: both
// are half of the only identity a Soulseek file has, and both are foreign keys.
func filterResults(responses []slskd.Response, downloaded map[string]struct{}) []keptResult {
	var kept []keptResult
	for _, resp := range responses {
		if resp.Username == "" {
			continue
		}
		for _, f := range resp.Files {
			if f.Filename == "" {
				continue
			}
			if _, taken := downloaded[fileKey(resp.Username, f.Filename)]; !taken {
				continue
			}
			kept = append(kept, keptResult{peer: resp.Username, file: f})
		}
	}
	return kept
}

// syncEvents replays slskd's persisted record of finished transfers, in both
// directions.
//
// Walked newest-first and stopped at the first page holding nothing new, the
// same shape as every other history walk here. The full log is tens of
// megabytes because each event embeds its whole transfer object, so re-reading
// it hourly would be the most expensive thing this service does for no gain.
//
// Each file is stat'd as it is recorded. That is the point of the pass: the
// local path only exists here, and its inode is what later ties the download to
// the library file it became. A file that has already been moved into the
// library cannot be stat'd at this path — statInto returns NULL for it — and
// that is the unrecoverable case, which is why the stat happens at the earliest
// moment the path is known rather than later.
// The walk resumes from a stored cursor when a sidecar is configured. Without
// one it restarts at offset 0 every pass and stops on the first page holding
// nothing new — which re-reads at least a page every time and cannot tell "the
// log is caught up" from "this page happened to be all-seen", so a burst of
// re-delivered events would end the walk early.
func (s *Slskd) syncEvents(ctx context.Context) (downloads, uploads int, err error) {
	observedAt := time.Now().UTC()

	if s.walk != nil {
		_, err := s.walk.Run(ctx, func(ctx context.Context, page []sidecar.Event) error {
			events, err := decodeSlskdEvents(page, s.logger)
			if err != nil {
				return err
			}
			d, u, _, err := s.writeEvents(ctx, events, observedAt)
			downloads += d
			uploads += u
			return err
		})
		return downloads, uploads, err
	}

	var walkErr error
	err = s.client.Events(ctx, eventPageSize, func(page []slskd.Event) bool {
		d, u, fresh, err := s.writeEvents(ctx, page, observedAt)
		downloads += d
		uploads += u
		if err != nil {
			walkErr = err
			return false
		}
		return fresh > 0
	})
	if walkErr != nil {
		return downloads, uploads, walkErr
	}
	return downloads, uploads, err
}

// writeEvents records one page of transfer events.
//
// Shared by both walks so the transfer-recording rules exist once: which event
// types count, that a malformed payload is skipped rather than fatal, and which
// direction a completion belongs to. Two copies of this would drift, and the
// drift would be invisible — both paths write into the same tables.
//
// fresh counts rows that were new to the store, which only the direct walk uses
// as its stop condition. The cursor walk has a real position and does not need
// to infer one.
func (s *Slskd) writeEvents(
	ctx context.Context,
	page []slskd.Event,
	observedAt time.Time,
) (downloads, uploads, fresh int, err error) {
	for _, ev := range page {
		switch ev.Type {
		case slskd.EventDownloadFileComplete, slskd.EventUploadFileComplete:
		default:
			continue
		}
		done, err := ev.AsFileComplete()
		if err != nil {
			// One malformed payload is not worth abandoning the walk;
			// slskd has changed this shape before.
			s.logger.WarnContext(ctx, "undecodable transfer event",
				"id", ev.ID, "type", ev.Type, "err", err)
			continue
		}

		var isNew bool
		if ev.Type == slskd.EventUploadFileComplete {
			isNew, err = s.writeUploadEvent(ctx, ev, done, observedAt)
			uploads++
		} else {
			isNew, err = s.writeDownloadEvent(ctx, done, observedAt)
			downloads++
		}
		if err != nil {
			return downloads, uploads, fresh, err
		}
		if isNew {
			fresh++
		}
	}
	s.logger.InfoContext(ctx, "slskd events progress",
		"downloads", downloads, "uploads", uploads, "new_this_page", fresh)
	return downloads, uploads, fresh, nil
}

// decodeSlskdEvents unwraps a page of sidecar events.
//
// A payload that will not decode is skipped with a warning rather than failing
// the page: slskd has changed this shape before, and one bad row should not
// stop a walk over 64,000 of them.
func decodeSlskdEvents(page []sidecar.Event, logger *slog.Logger) ([]slskd.Event, error) {
	out := make([]slskd.Event, 0, len(page))
	for _, e := range page {
		if e.Kind != slskdsvc.KindEvent {
			continue
		}
		ev, err := sidecar.DecodePayload[slskd.Event](e)
		if err != nil {
			logger.Warn("undecodable slskd event", "ref", e.Ref, "err", err)
			continue
		}
		out = append(out, ev)
	}
	return out, nil
}

// eventPageSize bounds one page of the event log.
//
// Smaller than the other walkers': every download event carries a full transfer
// object, so a page here is roughly a megabyte.
const eventPageSize = 500

// writeDownloadEvent records one finished download and reports whether it was
// new to the store.
func (s *Slskd) writeDownloadEvent(ctx context.Context, done slskd.FileComplete, observedAt time.Time) (bool, error) {
	username := done.Peer()
	if username == "" || done.RemoteFilename == "" {
		return false, nil
	}
	peer, err := s.lookups.SoulseekPeer(ctx, username)
	if err != nil || peer == nil {
		return false, err
	}

	var existing model.SoulseekDownload
	found, err := dao.FindOne(ctx, s.dao, &existing,
		"peer_username = ? AND remote_path = ?", *peer, done.RemoteFilename)
	if err != nil {
		return false, err
	}

	// Stat only what has not been recorded with an inode already. The path is
	// gone once the file has been moved into the library, so a second look
	// cannot improve on the first and would be an NFS round trip per event.
	inode, size := existing.FileDataInode, existing.FileDataSizeBytes
	if inode == nil && done.LocalFilename != "" {
		inode, size, err = statInto(ctx, s.dao, done.LocalFilename, observedAt)
		if err != nil {
			return false, err
		}
	}

	t := done.Transfer
	row := model.SoulseekDownload{
		ClientName:        s.clientName,
		PeerUsername:      *peer,
		RemotePath:        done.RemoteFilename,
		LocalPath:         strPtr(done.LocalFilename),
		FileDataInode:     inode,
		FileDataSizeBytes: size,
		State:             strPtr(t.State),
		Bytes:             int64Ptr(t.BytesTransferred),
		SizeBytes:         int64Ptr(t.Size),
		RequestedAt:       timePtr(t.RequestedAt.Time),
		StartedAt:         timePtr(t.StartedAt.Time),
		FinishedAt:        timePtr(t.EndedAt.Time),
	}
	if err := dao.Upsert(ctx, s.dao, []model.SoulseekDownload{row},
		[]string{"peer_username", "remote_path"},
		[]string{"client_name", "local_path", "file_data_inode",
			"file_data_size_bytes", "state", "bytes", "size_bytes",
			"requested_at", "started_at", "finished_at"},
	); err != nil {
		return false, fmt.Errorf("slskd: download event %q: %w", done.RemoteFilename, err)
	}
	return !found, nil
}

// writeUploadEvent records one file a peer took from this share.
//
// The only record of demand in the model. Everything else says what was wanted
// and obtained; this says what somebody else wanted out of what is already
// held. Keyed by slskd's own transfer id, which is unique across every upload
// it has recorded, so a re-read of the log cannot duplicate a row.
//
// The local path is a library path — slskd shares the library directly — so the
// stat here is what ties an upload to the file that was taken. Through the
// inode rather than the path, because an *arr rename changes one and not the
// other.
func (s *Slskd) writeUploadEvent(ctx context.Context, ev slskd.Event, done slskd.FileComplete, observedAt time.Time) (bool, error) {
	username := done.Peer()
	transferID := done.Transfer.ID
	if transferID == "" {
		// Without slskd's id there is no key. Falling back to the event id
		// would be a different identity for the same transfer if the payload
		// were ever re-emitted.
		return false, nil
	}
	if username == "" || done.RemoteFilename == "" {
		return false, nil
	}
	peer, err := s.lookups.SoulseekPeer(ctx, username)
	if err != nil || peer == nil {
		return false, err
	}

	var existing model.SoulseekUpload
	found, err := dao.FindOne(ctx, s.dao, &existing, "transfer_id = ?", transferID)
	if err != nil {
		return false, err
	}
	if found {
		// An upload is finished and immutable. Re-reading the log cannot learn
		// anything new about one, so there is nothing to update.
		return false, nil
	}

	inode, size, err := statInto(ctx, s.dao, done.LocalFilename, observedAt)
	if err != nil {
		return false, err
	}

	t := done.Transfer
	finished := timePtr(t.EndedAt.Time)
	if finished == nil {
		// Fall back to the event's own timestamp: the payload is a completion,
		// so it happened, and finished_at is what a popularity question sorts
		// and buckets by.
		finished = timePtr(ev.Timestamp)
	}

	row := model.SoulseekUpload{
		TransferID:        transferID,
		ClientName:        s.clientName,
		PeerUsername:      *peer,
		LocalPath:         strPtr(done.LocalFilename),
		RemotePath:        done.RemoteFilename,
		FileDataInode:     inode,
		FileDataSizeBytes: size,
		SizeBytes:         int64Ptr(t.Size),
		Bytes:             int64Ptr(t.BytesTransferred),
		State:             strPtr(t.State),
		RequestedAt:       timePtr(t.RequestedAt.Time),
		StartedAt:         timePtr(t.StartedAt.Time),
		FinishedAt:        finished,
	}
	if err := dao.Insert(ctx, s.dao, []model.SoulseekUpload{row},
		[]string{"transfer_id"}); err != nil {
		return false, fmt.Errorf("slskd: upload event %s: %w", transferID, err)
	}
	return true, nil
}
