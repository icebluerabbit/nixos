// Package ingest persists webhook bodies.
//
// Webhooks are enrichment, never the authority. A reconcile pass reads a full
// snapshot and is correct after a restart; a webhook missed while this process
// was down is gone forever. So nothing here is the only way a fact can be
// learned, with one deliberate exception: autobrr's infohash, which its own API
// does not serialise and which therefore arrives here or not at all.
//
// What the handlers are actually for is the gap between passes. A torrent added
// and removed inside one hour, an extraction that failed, a cross-seed
// injection — the hourly snapshot sees none of those, because by the time it
// runs the evidence is gone.
//
// Every handler is best-effort by construction. A parse failure is logged and
// dropped rather than returned: the sender is a service that will disable a
// webhook target that answers with an error, and losing the whole stream
// because one payload changed shape is a worse outcome than losing one event.
package ingest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
)

// Config names the service instances a webhook is attributed to.
//
// A payload says what happened but never which deployment said it, and the
// model keys on instance rather than software — two Sonarrs are two sources of
// truth whose ids share no namespace. The names must match the ones the
// reconcile passes use, or a webhook would create a second Service row for the
// same thing.
type Config struct {
	Sonarr        string
	Radarr        string
	Lidarr        string
	Prowlarr      string
	Autobrr       string
	CrossSeed     string
	Seasonpackarr string
	Unpackerr     string
	QBittorrent   string
	Slskd         string
}

// Ingest routes and persists webhook bodies.
type Ingest struct {
	dao     *dao.DAO
	lookups *dao.Lookups
	logger  *slog.Logger
	cfg     Config
}

// New builds an ingester.
func New(d *dao.DAO, l *dao.Lookups, logger *slog.Logger, cfg Config) *Ingest {
	return &Ingest{dao: d, lookups: l, logger: logger, cfg: cfg}
}

// Handle persists one delivery, which may carry more than one event.
//
// The signature returns nothing deliberately — see the package comment. The
// server has already answered 200 by the time a failure here is known, and
// there is nowhere for an error to go but the log.
//
// A delivery is split before anything looks at it, because not every sender
// posts a single object. The Vector sidecars batch: slskd and soularr have no
// webhook of their own, so their events arrive as journal lines Vector remapped
// and shipped together, concatenated rather than wrapped in an array. Decoding
// the whole body as one value fails on the second object with "invalid
// character '{' after top-level value" and drops the entire batch — the first
// event included, which had parsed perfectly well.
func (i *Ingest) Handle(ctx context.Context, source string, body []byte) {
	events, err := split(body)
	if err != nil {
		i.logger.ErrorContext(ctx, "ingest failed", "source", source, "err", err)
		return
	}
	for _, event := range events {
		i.handleOne(ctx, source, event)
	}
}

// split separates a delivery into the JSON values it carries.
//
// One object is the common case and stays a single-element result. Several
// concatenated — with or without newlines between them — become one element
// each, which is what a batching sender produces.
//
// A value that will not parse ends the split with an error rather than being
// skipped: a body this cannot read is a defect here or a change at the sender,
// and silently dropping it would turn that into a gap nobody notices.
func split(body []byte) ([][]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var out [][]byte
	for {
		var raw json.RawMessage
		err := dec.Decode(&raw)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}
		out = append(out, raw)
	}
}

// handleOne persists a single event.
func (i *Ingest) handleOne(ctx context.Context, source string, body []byte) {
	var err error
	switch source {
	case "autobrr":
		err = i.autobrr(ctx, body)
	case "sonarr":
		err = i.arr(ctx, i.cfg.Sonarr, body)
	case "radarr":
		err = i.arr(ctx, i.cfg.Radarr, body)
	case "lidarr":
		err = i.arr(ctx, i.cfg.Lidarr, body)
	case "prowlarr":
		err = i.prowlarr(ctx, body)
	case "qbittorrent":
		err = i.qbittorrent(ctx, body)
	case "cross-seed":
		err = i.crossSeed(ctx, body)
	case "unpackerr":
		err = i.unpackerr(ctx, body)
	case "seasonpackarr":
		err = i.seasonpackarr(ctx, body)
	case "slskd":
		err = i.slskd(ctx, body)
	case "soularr":
		err = i.soularr(ctx, body)
	default:
		// Not an error. Several sources are wired to a path and carry nothing
		// this model has a column for: qbt-watch reports a filename with no
		// infohash, and jellyfin and readarr are out of scope. The server
		// still logs and counts them.
		return
	}
	if err != nil {
		i.logger.ErrorContext(ctx, "ingest failed", "source", source, "err", err)
	}
}

// decode unmarshals a body into v.
//
// Wrapped only to keep every handler's first three lines from being the same
// three lines.
func decode(body []byte, v any) error {
	return json.Unmarshal(body, v)
}

// Pointer helpers, duplicated from the reconcile package rather than shared.
//
// They are four lines each and sharing them would mean an internal package
// existing solely to hold them, imported by both. The rule they encode — zero
// means absent for the counts and identifiers this model carries — is stated in
// reconcile and is the same here.

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func intPtr(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func int64Ptr(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

func boolPtr(v bool) *bool { return &v }

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	u := t.UTC()
	return &u
}

// parseBytes reads a size that a sender rendered as a string.
//
// Shell hooks and template-driven webhooks send every field as text, so a byte
// count arrives as "1234" rather than 1234. Unparseable means absent.
func parseBytes(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
