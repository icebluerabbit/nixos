package ingest

import (
	"context"
	"fmt"
	"strings"
	"time"

	"homelab/managarr/internal/dao"
	"homelab/managarr/internal/model"
)

// seasonpackarrHook is the embed seasonpackarr's notifier posts.
//
// Upstream calls it a Discord notifier and performs no URL validation, so it is
// pointed at the Vector sidecar and forwarded here verbatim. The embed's title
// is BuildTitle(statusCode) — the upstream StatusCode enum rendered as prose —
// which makes it a bounded label rather than a message to pattern-match.
type seasonpackarrHook struct {
	Embeds []struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Fields      []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"fields"`
	} `json:"embeds"`
}

// seasonpackarr records one pack attempt and its outcome.
//
// seasonpackarr asks "does the client already hold the episodes this season
// pack contains, and can they be hardlinked into it" — and answers no far more
// often than yes. The refusals are the point: "resolution did not match",
// "number of matches below threshold", "could not create hardlinks" are the
// reasons a season pack was announced and not taken, and they exist in no API.
func (i *Ingest) seasonpackarr(ctx context.Context, body []byte) error {
	// Two shapes arrive on this route. The notifier's Discord embed, and — since
	// the embed proved to fire for almost nothing, 1 of 19 assembled packs — the
	// structured log line, forwarded by the container's Vector transform.
	//
	// Distinguished by their own fields rather than by a wrapper: the embed has
	// "embeds", the log line has "module". Neither has the other.
	if logEvent, ok := seasonpackarrLogEvent(body); ok {
		return i.seasonpackarrLog(ctx, logEvent)
	}

	var hook seasonpackarrHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("seasonpackarr: decode: %w", err)
	}
	if len(hook.Embeds) == 0 {
		return nil
	}
	embed := hook.Embeds[0]
	if embed.Title == "" {
		return nil
	}

	// Field labels are prose, and seasonpackarr writes "Release Name" — not
	// "Release". Matching the wrong label is silent: the embed decodes, the
	// title is read, and the attempt is dropped for want of a key. Every label
	// this notifier has been observed to use is listed rather than guessed at.
	var release, client string
	for _, f := range embed.Fields {
		switch strings.ToLower(f.Name) {
		case "release name", "release", "torrent", "name":
			release = f.Value
		case "client":
			client = f.Value
		}
	}
	if release == "" {
		// The description carries the release name on the builds that do not
		// send a field for it.
		release = strings.TrimSpace(embed.Description)
	}
	if release == "" {
		// pack_attempts is keyed by release. Without one there is nothing to
		// record the attempt against.
		return nil
	}

	releaseID, err := i.lookups.Release(ctx, release)
	if err != nil || releaseID == nil {
		return err
	}

	return dao.Upsert(ctx, i.dao, []model.PackAttempt{{
		ReleaseID:   *releaseID,
		AttemptedAt: time.Now().UTC(),
		// The title verbatim, lowercased and underscored: it is the upstream
		// enum, so normalising it keeps it a label rather than turning it into
		// prose that has to be matched later.
		Outcome: strPtr(strings.ReplaceAll(strings.ToLower(embed.Title), " ", "_")),
		Reason:  strPtr(embed.Description),
		Client:  strPtr(client),
	}}, []string{"release_id", "attempted_at"},
		[]string{"outcome", "reason", "client"})
}

// seasonpackarrLog is one line of seasonpackarr's structured log, as forwarded
// by the container's Vector transform.
//
// The notifier was the original source for pack attempts and turned out to
// record almost nothing: 1 of the 19 assembled packs on disk had an attempt
// against it. The log records every announce it considered, every candidate it
// rejected and why, and the match ratio — none of which the embed carries.
type seasonpackarrLog struct {
	// Release is the season pack that was announced.
	Release string `json:"release"`
	Client  string `json:"clientname"`
	// Error is set on the lines that end an attempt.
	Error string `json:"error"`
	// Message is the prose. Its shapes are enumerated in seasonpackarrLog's
	// handler rather than pattern-matched loosely.
	Message string `json:"message"`
	// Module is how a log line is told from the notifier's embed.
	Module string `json:"module"`
}

// seasonpackarrLogEvent decodes a log line, reporting whether the body was one.
func seasonpackarrLogEvent(body []byte) (seasonpackarrLog, bool) {
	var ev seasonpackarrLog
	if err := decode(body, &ev); err != nil {
		return ev, false
	}
	// module is present on every processor line and on nothing the notifier
	// sends. A release is also required: the lines without one are lifecycle
	// noise ("starting to process season pack request") with nothing to record
	// an attempt against.
	if ev.Module == "" || ev.Release == "" {
		return ev, false
	}
	return ev, true
}

// Outcomes seasonpackarr reports, as its own error strings.
//
// Verbatim rather than normalised at the source: these are the strings it
// writes, and matching them exactly means a changed one stops matching loudly
// instead of being absorbed by a looser pattern.
var seasonpackarrOutcomes = map[string]string{
	"no matching releases in client":    "no_matching_releases_in_client",
	"number of matches below threshold": "number_of_matches_below_threshold",
	"release already in client":         "release_already_in_client",
}

// seasonpackarrLog records what one log line says about an attempt.
//
// Only the lines that CONCLUDE an attempt are recorded as attempts. The
// per-candidate rejections are far more numerous — a single announce produces
// one per torrent it looked at — and recording each as an attempt would report
// twenty failures for one announce.
func (i *Ingest) seasonpackarrLog(ctx context.Context, ev seasonpackarrLog) error {
	outcome, ok := seasonpackarrOutcomes[ev.Error]
	if !ok {
		// The success path has no error field; it says so in the message.
		switch ev.Message {
		case "successfully matched season pack to episodes in client",
			"successfully parsed torrent and hardlinked episodes":
			outcome = "successful_match"
		default:
			// A candidate rejection, a match ratio, or a line this build does
			// not recognise. Not an attempt outcome, so not recorded as one.
			return nil
		}
	}

	releaseID, err := i.lookups.Release(ctx, ev.Release)
	if err != nil || releaseID == nil {
		return err
	}

	return dao.Upsert(ctx, i.dao, []model.PackAttempt{{
		ReleaseID:   *releaseID,
		AttemptedAt: time.Now().UTC(),
		Outcome:     strPtr(outcome),
		Reason:      strPtr(ev.Message),
		Client:      strPtr(ev.Client),
	}}, []string{"release_id", "attempted_at"},
		[]string{"outcome", "reason", "client"})
}
