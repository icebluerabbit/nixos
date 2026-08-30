package actions

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"mousetrap/app"
	"mousetrap/mam"
	"mousetrap/policy"
	"mousetrap/store"
	"mousetrap/types"
)

// Shop refreshes the account, applies the bonus-point spend policy, and (unless
// dry-run) executes the buy via bonusBuy — recording the point spend. Timers hit
// this on a schedule.
func Shop(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if err := a.State.RefreshAccount(ctx); err != nil {
			a.Metrics.MAMError("loaduser")
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		acct := a.State.Account()
		since := time.Now().Add(-a.Cfg.Grab.InFlightWindow).Unix()
		inflight, _ := a.Store.InFlightSince(ctx, since)
		action, plan := a.Policy.Shop(acct, inflight)

		executed := false
		if action != types.ShopHold && !a.Cfg.Shop.DryRun {
			if err := execShop(ctx, a, action, plan); err != nil {
				slog.Error("shop", "action", action, "err", err.Error())
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			executed = true
		}
		slog.Info("shop",
			"action", action, "dryRun", a.Cfg.Shop.DryRun, "executed", executed,
			"points", acct.SeedBonus, "cost", plan.Cost)
		w.WriteHeader(http.StatusOK)
	}
}

// execShop performs the buy and records the point spend.
func execShop(ctx context.Context, a *app.App, action types.ShopAction, plan policy.ShopPlan) error {
	switch action {
	case types.ShopBuyVIP:
		if _, err := a.MAM.BonusBuy(ctx, mam.BuyParams{SpendType: mam.SpendVIP, Duration: "max"}); err != nil {
			a.Metrics.MAMError("buy")
			return err
		}
		notify(a, types.WebhookSpend, map[string]any{
			"type": string(types.SpendPoints), "amount": plan.Cost, "reason": string(types.SpendReasonShopVIP),
		})
		_, err := a.Store.InsertSpend(ctx, store.Spend{
			Type: types.SpendPoints, Amount: plan.Cost, Reason: types.SpendReasonShopVIP,
		})
		return err
	case types.ShopBuyUpload:
		if _, err := a.MAM.BonusBuy(ctx, mam.BuyParams{
			SpendType: mam.SpendUpload, Amount: strconv.FormatInt(plan.Amount, 10),
		}); err != nil {
			a.Metrics.MAMError("buy")
			return err
		}
		notify(a, types.WebhookSpend, map[string]any{
			"type": string(types.SpendPoints), "amount": plan.Cost, "reason": string(types.SpendReasonShopUpload),
		})
		_, err := a.Store.InsertSpend(ctx, store.Spend{
			Type: types.SpendPoints, Amount: plan.Cost, Reason: types.SpendReasonShopUpload,
		})
		return err
	}
	return nil
}
