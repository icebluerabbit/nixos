package actions

import (
	"log/slog"
	"net/http"

	"mousetrap/app"
	"mousetrap/mam"
	"mousetrap/store"
	"mousetrap/types"
)

// bonusTypes is the full MAM userBonusHistory type set mirrored into the ledger.
var bonusTypes = []string{
	string(types.BonusGiftPoints), string(types.BonusGiftWedge),
	string(types.BonusWedgePF), string(types.BonusWedgeGFL),
	string(types.BonusTorrentThanks), string(types.BonusMillionaires),
}

// Account refreshes the account snapshot (ratio, uploaded/downloaded, unsat, wedges,
// class, points) from MAM into the state service. Timers hit this on a schedule.
func Account(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := a.State.RefreshAccount(r.Context()); err != nil {
			a.Metrics.MAMError("loaduser")
			slog.Error("refresh_error", "endpoint", "account", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}
}

// Freeleech queries the freeleech inventory and logs it. No state, no storage — the
// freeleech set isn't account-specific and has no consumer yet.
func Freeleech(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := a.MAM.Search(r.Context(), mam.SearchParams{
			SearchType: "fl",
			MainCat:    []int{13, 14}, // AudioBooks, E-Books
			SortType:   "dateDesc",
			PerPage:    100,
		})
		if err != nil {
			a.Metrics.MAMError("search")
			slog.Error("refresh_error", "endpoint", "freeleech", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		slog.Info("freeleech", "total_found", res.TotalFound, "count", len(res.Data))
		w.WriteHeader(http.StatusOK)
	}
}

// Bonus mirrors new MAM userBonusHistory rows into the bonus ledger, deduping by the
// stored cursor (MAX(ts)). Feeds the received/history metrics.
func Bonus(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		cursor, err := a.Store.BonusCursor(ctx)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rows, err := a.MAM.BonusHistory(ctx, mam.BonusParams{Type: bonusTypes})
		if err != nil {
			a.Metrics.MAMError("bonus")
			slog.Error("refresh_error", "endpoint", "bonus", "err", err.Error())
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		inserted := 0
		for _, e := range rows {
			if e.Timestamp <= cursor {
				continue
			}
			if _, err := a.Store.InsertBonus(ctx, store.Bonus{
				TS:          e.Timestamp,
				Type:        types.BonusType(e.Type),
				Amount:      int64(e.Amount),
				TID:         e.TID,
				OtherUserID: e.OtherUserID,
				OtherName:   e.OtherName,
			}); err == nil {
				inserted++
				notify(a, types.WebhookBonusReceived, map[string]any{
					"type": e.Type, "amount": e.Amount, "tid": e.TID, "other_name": e.OtherName,
				})
			}
		}
		slog.Info("bonus", "fetched", len(rows), "inserted", inserted)
		w.WriteHeader(http.StatusOK)
	}
}
