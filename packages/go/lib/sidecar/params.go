package sidecar

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"homelab/httpx"
)

// httpxConfig builds the relay's client config.
//
// managarr's ingest is on the same wired LAN and does nothing slow — it writes
// a row — so the default retry policy is the right one and there is nothing to
// tune here beyond naming the sender.
func httpxConfig(baseURL string, logger *slog.Logger) httpx.Config {
	return httpx.Config{
		BaseURL:   baseURL,
		Logger:    logger,
		UserAgent: "managarr-sidecar",
	}
}

// intParam reads a non-negative integer query parameter.
func intParam(r *http.Request, name string, def int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("%s must be a non-negative integer", name)
	}
	return v, nil
}

// writeJSON sends a result, or a 500 when producing it failed.
func writeJSON(w http.ResponseWriter, log *slog.Logger, v any, err error) {
	if err != nil {
		writeError(w, log, http.StatusInternalServerError, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status is already sent, so this can only be logged.
		log.Error("encode response failed", "err", err)
	}
}

// writeError answers with a status and a plain reason.
func writeError(w http.ResponseWriter, log *slog.Logger, status int, err error) {
	log.Error("request failed", "status", status, "err", err)
	http.Error(w, err.Error(), status)
}
