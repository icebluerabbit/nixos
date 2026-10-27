package reconcile

import (
	"context"
	"fmt"

	"homelab/clients/arr"
	"homelab/sidecar"
	"homelab/sidecar/arrsvc"
)

// historyRead is one pass's worth of *arr history, plus how to commit it.
//
// The two are inseparable and that is the point: the records are only safe to
// mark as read once the work that consumes them has succeeded, so handing back
// a slice without the commit would make it possible to forget.
type historyRead struct {
	// commit runs fn over the records and advances the cursor only if fn
	// returned nil. It reports the cursor the read started from — empty on a
	// first run or when there is no sidecar.
	//
	// Returned rather than held on the struct because on the sidecar path the
	// read has not happened yet when this is built: RunAtomic does it, and a
	// field set inside the closure would still be empty to a caller that read
	// it before calling commit.
	commit func(context.Context, func(context.Context, []arr.HistoryRecord) error) (string, error)
}

// history reads the instance's history, through the sidecar when one is
// configured and directly otherwise.
//
// Both paths exist on purpose. The sidecar is what makes the pass incremental —
// it synthesizes the since-cursor the *arr does not offer — but a sidecar is
// one more thing that can be down, and the *arr's own /history is always there.
// Falling back means a sidecar outage costs the cursor, not the pass.
func (a *Arr) history(ctx context.Context) (historyRead, error) {
	if a.walk == nil {
		records, err := a.client.History(ctx, a.pageSize)
		if err != nil {
			return historyRead{}, err
		}
		return historyRead{
			commit: func(ctx context.Context, fn func(context.Context, []arr.HistoryRecord) error) (string, error) {
				return "", fn(ctx, records)
			},
		}, nil
	}

	// Deferred: RunAtomic does the reading, and it only advances the cursor
	// once fn has returned. Doing the read here and the commit later would put
	// the two on either side of a return, which is exactly how a cursor ends
	// up ahead of the work.
	return historyRead{
		commit: func(ctx context.Context, fn func(context.Context, []arr.HistoryRecord) error) (string, error) {
			st, err := a.walk.RunAtomic(ctx, func(ctx context.Context, events []sidecar.Event) error {
				records, err := decodeHistory(events)
				if err != nil {
					return err
				}
				return fn(ctx, records)
			})
			return st.Resumed, err
		},
	}, nil
}

// decodeHistory unwraps a page of sidecar events into *arr history records.
//
// Events of another kind are skipped rather than refused: managarr-sonarr also
// emits enriched webhooks on the same stream, and a consumer that died on the
// first kind it did not recognise would make every addition a breaking change.
func decodeHistory(events []sidecar.Event) ([]arr.HistoryRecord, error) {
	out := make([]arr.HistoryRecord, 0, len(events))
	for _, e := range events {
		if e.Kind != arrsvc.KindHistory {
			continue
		}
		r, err := sidecar.DecodePayload[arr.HistoryRecord](e)
		if err != nil {
			return nil, fmt.Errorf("arr: decode history event: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}
