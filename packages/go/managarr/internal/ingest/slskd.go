package ingest

import (
	"context"
	"fmt"

	"homelab/managarr/internal/dao"
)

// slskdHook is the event slskd's Vector sidecar emits.
//
// slskd has no webhook, so this is a journal line the sidecar recognised and
// re-encoded. The shape is the remap's, not a vendor schema:
//
//	{"source":"slskd","event":"...","file":"...","user":"...","reason":"..."}
type slskdHook struct {
	Source string `json:"source"`
	Event  string `json:"event"`
	File   string `json:"file"`
	User   string `json:"user"`
	Reason string `json:"reason"`
}

// slskd records a transfer event.
//
// The API expires a completed or failed transfer on its own schedule, so the
// hourly pass sees only what is still in flight. A download that failed and
// aged out between two passes leaves nothing behind — and the failure reason is
// the whole value of the Soulseek half of the record, since a peer going
// offline mid-transfer is the normal outcome there rather than an exception.
//
// Only existing rows are updated. A transfer this has never seen through the
// API cannot be created from an event alone: soulseek_downloads is keyed by
// peer and remote path and requires a client, and a journal line saying a file
// failed is not evidence of which client asked for it.
func (i *Ingest) slskd(ctx context.Context, body []byte) error {
	var hook slskdHook
	if err := decode(body, &hook); err != nil {
		return fmt.Errorf("slskd: decode: %w", err)
	}
	if hook.User == "" || hook.File == "" {
		return nil
	}

	sd := i.dao.Q().SoulseekDownload
	res, err := sd.WithContext(ctx).
		Where(sd.PeerUsername.Eq(hook.User), sd.RemotePath.Eq(hook.File)).
		UpdateSimple(
			dao.Assign(sd.State, strPtr(hook.Event)),
			dao.Assign(sd.Error, strPtr(hook.Reason)),
		)
	if err != nil {
		return fmt.Errorf("slskd: update transfer: %w", err)
	}
	if res.RowsAffected == 0 {
		i.logger.DebugContext(ctx, "slskd event for unknown transfer",
			"user", hook.User, "file", hook.File, "event", hook.Event)
	}
	return nil
}
