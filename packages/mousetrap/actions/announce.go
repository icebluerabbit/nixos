package actions

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mousetrap/app"
	"mousetrap/client"
	"mousetrap/mam"
	"mousetrap/policy"
	"mousetrap/store"
	"mousetrap/types"
)

// grabTimeout bounds the account refresh + grab so a slow/hung MAM can't hang the
// autobrr webhook.
const grabTimeout = 15 * time.Second

// announcePayload is the autobrr webhook body (the MAM filter's webhook_data). The
// field names here say what the data is; the autobrr macros they are fed from do not
// (see the mapping table in the filter's _mam.nix).
type announcePayload struct {
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	MediaType    string   `json:"mediaType"`    // "Ebook" | "Audiobook" | "Periodical Ebook"
	Categories   string   `json:"categories"`   // comma-separated; multi-valued
	Filetypes    string   `json:"filetypes"`    // comma-separated; multi-valued
	Fiction      string   `json:"fiction"`      // "Fiction" | "Non-Fiction"
	Language     []string `json:"language"`     // autobrr .Language (rls; empty on m4b)
	LanguageName string   `json:"languageName"` // parsed from the name suffix (reliable)
	Size         string   `json:"size"`         // bytes as string
	VIP          string   `json:"vip"`
	Freeleech    string   `json:"freeleech"`
	TorrentID    string   `json:"torrentId"`
	DownloadURL  string   `json:"downloadUrl"`
}

// Announce handles the autobrr webhook synchronously: dedup → Layer-1 accept →
// record the announce → (on accept) refresh the account snapshot → Layer-2 grab-gate
// → record the grab. The HTTP status reflects the real outcome (200 only when
// actually grabbed), so autobrr's external-filter expected status should be 200, and
// autobrr must NOT also have a download action on this filter (mousetrap grabs).
func Announce(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		// dump the exact bytes autobrr sent, before any parsing.
		slog.Info("announce_raw", "body", string(body))

		var p announcePayload
		if err := json.Unmarshal(body, &p); err != nil {
			http.Error(w, "bad payload", http.StatusBadRequest)
			return
		}
		rel := p.release()
		ctx := r.Context()

		// dedup: already grabbed this tid → don't re-process.
		if seen, err := a.Store.GrabSeen(ctx, rel.TID); err == nil && seen {
			respond(w, http.StatusNotAcceptable, types.Reject, types.AcceptDuplicate, types.ResultNone)
			return
		}

		decision, reason := a.Policy.Accept(rel)
		annID, _ := a.Store.InsertAnnounce(ctx, announceRow(rel, decision, reason))

		grabDecision, grabReason := types.Skip, types.GrabLayer1Reject
		result := types.ResultNone
		if decision == types.Accept {
			gctx, cancel := context.WithTimeout(ctx, grabTimeout)
			defer cancel()

			// Refresh the snapshot before taking the grab lock, so the slow MAM poll
			// doesn't serialize behind it. Proceed with the last snapshot on error.
			if err := a.State.RefreshAccount(gctx); err != nil {
				a.Metrics.MAMError("loaduser")
			}

			// Serialized grab: the in-flight read, gate, and grab record are atomic.
			func() {
				a.GrabMu.Lock()
				defer a.GrabMu.Unlock()

				acct := a.State.Account()
				since := time.Now().Add(-a.Cfg.Grab.InFlightWindow).Unix()
				inflight, _ := a.Store.InFlightSince(gctx, since)

				gd, gr, plan := a.Policy.Grab(rel, acct, inflight)
				grabDecision, grabReason = gd, gr
				_ = a.Store.UpdateAnnounceGrab(gctx, annID, string(gd), string(gr))
				if gd == types.Grab {
					result = execGrab(gctx, a, rel, annID, plan)
				}
			}()
		}

		slog.Info("announce",
			"tid", rel.TID, "name", rel.Name,
			// joined for the log line only — Loki's json parser flattens an array to
			// categories_0/categories_1, which no line_format can render.
			"media_type", rel.MediaType, "categories", strings.Join(rel.Categories, ", "),
			"fiction", rel.Fiction,
			"decision", decision, "reason", reason,
			"grab_decision", grabDecision, "grab_reason", grabReason, "grab_result", result)

		status := http.StatusNotAcceptable
		if result == types.ResultAdded || result == types.ResultAddedWedge {
			status = http.StatusOK
		}
		respond(w, status, decision, reason, result)

		// Notify after responding so autobrr never waits on the webhook POST.
		notify(a, types.WebhookAnnounce, map[string]any{
			"tid": rel.TID, "name": rel.Name, "media_type": rel.MediaType,
			"categories": rel.Categories, "fiction": rel.Fiction,
			"filetypes": rel.Filetypes, "languages": rel.Languages, "size": rel.Size,
			"vip": rel.VIP, "freeleech": rel.Freeleech,
		})
		switch {
		case decision == types.Reject:
			notify(a, types.WebhookReject, map[string]any{
				"tid": rel.TID, "name": rel.Name, "media_type": rel.MediaType,
				"categories": rel.Categories, "fiction": rel.Fiction,
				"reason": string(reason),
			})
		case grabDecision == types.Grab:
			notify(a, types.WebhookGrab, map[string]any{
				"tid": rel.TID, "name": rel.Name, "media_type": rel.MediaType,
				"categories": rel.Categories,
				"size":       rel.Size, "result": string(result),
			})
			if result == types.ResultAdded || result == types.ResultAddedWedge {
				notify(a, types.WebhookEnqueue, map[string]any{
					"tid": rel.TID, "name": rel.Name, "media_type": rel.MediaType, "result": string(result),
				})
			}
			if result == types.ResultAddedWedge {
				notify(a, types.WebhookSpend, map[string]any{
					"type": string(types.SpendWedge), "amount": 1,
					"reason": string(types.SpendReasonGrab), "tid": rel.TID,
				})
			}
		default: // accepted but held or skipped (unsat/buffer/vip)
			notify(a, types.WebhookHold, map[string]any{
				"tid": rel.TID, "name": rel.Name, "media_type": rel.MediaType,
				"categories":    rel.Categories,
				"grab_decision": string(grabDecision), "reason": string(grabReason),
			})
		}
	}
}

// notify fires a webhook for the event, logging (never propagating) any error — a
// notification failure must not affect the grab or the HTTP response.
func notify(a *app.App, event types.WebhookEvent, payload map[string]any) {
	if err := a.Webhook.Notify(event, payload); err != nil {
		slog.Warn("webhook", "event", string(event), "err", err.Error())
	}
}

// execGrab performs the grab and records it (and the wedge spend, if any). Runs under
// the grab lock. Dry-run records the decision only. A wedge is spent at download time
// (usetoken=fl), so a successful download records the spend regardless of whether the
// subsequent client add succeeds.
func execGrab(ctx context.Context, a *app.App, rel types.Release, annID int64, plan policy.GrabPlan) types.GrabResult {
	if a.Cfg.Grab.DryRun {
		recordGrab(ctx, a, rel, annID, plan, types.ResultDryRun, nil)
		return types.ResultDryRun
	}

	defer func(start time.Time) { a.Metrics.ObserveGrab(time.Since(start)) }(time.Now())

	data, err := a.MAM.Download(ctx, rel.TID, plan.UseWedge)
	if err != nil {
		a.Metrics.MAMError("download")
		logGrabError(rel, err)
		recordGrab(ctx, a, rel, annID, plan, types.ResultFetchFailed, nil)
		return types.ResultFetchFailed
	}

	// Download succeeded → the wedge (if used) is now spent; record it before the add.
	var spendID *int64
	if plan.UseWedge {
		if sid, err := a.Store.InsertSpend(ctx, store.Spend{
			Type: types.SpendWedge, TID: &rel.TID, Reason: types.SpendReasonGrab,
		}); err == nil {
			spendID = &sid
		}
	}

	result := types.ResultAdded
	if plan.UseWedge {
		result = types.ResultAddedWedge
	}
	if err := a.Client.Add(ctx, data, rel.Name, rel.MediaType); err != nil {
		result = types.ResultAddFailed
		if errors.Is(err, client.ErrRejected) {
			result = types.ResultAddRejected
		}
		slog.Error("grab", "tid", rel.TID, "result", result, "err", err.Error())
	} else {
		slog.Info("grab", "tid", rel.TID, "name", rel.Name, "result", result, "wedge", plan.UseWedge)
	}
	recordGrab(ctx, a, rel, annID, plan, result, spendID)
	return result
}

// logGrabError logs a fetch failure, including MAM's HTTP status and response body
// (e.g. the 406 "Account issue") when available.
func logGrabError(rel types.Release, err error) {
	attrs := []any{"tid", rel.TID, "stage", "fetch", "err", err.Error()}
	var he *mam.HTTPError
	if errors.As(err, &he) {
		attrs = append(attrs, "code", he.Code, "body", he.Body)
	}
	slog.Error("grab_error", attrs...)
}

// recordGrab persists the grab row linking the announce and any wedge spend.
func recordGrab(ctx context.Context, a *app.App, rel types.Release, annID int64, plan policy.GrabPlan, result types.GrabResult, spendID *int64) {
	_, _ = a.Store.InsertGrab(ctx, store.Grab{
		AnnounceID: &annID,
		SpendID:    spendID,
		Source:     types.SourceAnnounce,
		TID:        rel.TID,
		Size:       rel.Size,
		CostsRatio: plan.CostsRatio,
		Result:     string(result),
	})
}

func respond(w http.ResponseWriter, status int, decision types.AcceptDecision, reason types.AcceptReason, result types.GrabResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"decision": decision,
		"reason":   reason,
		"result":   result,
	})
}

func announceRow(rel types.Release, d types.AcceptDecision, reason types.AcceptReason) store.Announce {
	return store.Announce{
		TID:            rel.TID,
		Name:           rel.Name,
		MediaType:      rel.MediaType,
		Categories:     rel.Categories,
		Filetypes:      rel.Filetypes,
		Fiction:        rel.Fiction,
		Languages:      rel.Languages,
		Size:           rel.Size,
		VIP:            rel.VIP,
		Freeleech:      rel.Freeleech,
		AcceptDecision: string(d),
		AcceptReason:   string(reason),
	}
}

// release normalizes the autobrr payload onto the policy's domain Release.
func (p announcePayload) release() types.Release {
	size, _ := strconv.ParseInt(p.Size, 10, 64)
	name := p.Name
	if name == "" {
		name = p.Title
	}
	return types.Release{
		TID:         parseInt64(p.TorrentID),
		Name:        name,
		MediaType:   p.MediaType,
		Categories:  splitList(p.Categories),
		Filetypes:   splitList(p.Filetypes),
		Fiction:     p.Fiction,
		Languages:   reduceLangs(p.Language, p.LanguageName),
		Size:        size,
		VIP:         truthy(p.VIP),
		Freeleech:   truthy(p.Freeleech),
		DownloadURL: p.DownloadURL,
	}
}

// splitList splits one of MAM's multi-valued fields (categories, filetypes) into the
// list it represents. They arrive as one comma-separated string, not a JSON array,
// because autobrr's template macro re-joins them (domain/macros.go: Tags is a string
// built with strings.Join, and the []string Categories macro field is only ever
// populated by the RSS path). Languages need none of this — that macro field stays
// []string and arrives as a real array. This is the one boundary where the string
// becomes a list; everything downstream of it is []string.
func splitList(s string) []string {
	out := make([]string, 0, 4)
	for _, p := range strings.Split(s, ",") {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseInt64(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }

// normLang lowercases and trims a single language token.
func normLang(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// reduceLangs collapses to a single language set: autobrr's list, or the name-parsed
// fallback when the list is empty (rls drops the language on m4b). Normalized.
func reduceLangs(langs []string, name string) []string {
	if l := normLangs(langs); len(l) > 0 {
		return l
	}
	return normLangs([]string{name})
}

// normLangs normalizes a language list (lowercase + trim), dropping empties; nil if none.
func normLangs(langs []string) []string {
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		if n := normLang(l); n != "" {
			out = append(out, n)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func truthy(s string) bool {
	switch strings.ToLower(s) {
	case "true", "1", "yes":
		return true
	}
	return false
}
