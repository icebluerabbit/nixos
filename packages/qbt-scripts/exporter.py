"""Aggregate qBittorrent per-peer choke/interest state into global gauges.

The WebAPI exposes no global peer list, so peer flags are only reachable
per-torrent via /sync/torrentPeers. Cost scales with ACTIVE torrents
(filter=active), not the library size; a top-by-upspeed cap plus a wall-clock
budget bound the worst case, and both are reported so the scrape is never
silently sampled. Emits one JSON line to stdout; Vector parses it into gauges.

Peer flag vocabulary (qBittorrent):
  U = peer interested + we unchoked them (uploading)
  u = peer interested + we choked them (upload demand we refuse)
  D = downloading
  O = optimistic unchoke
  S = snubbed
  I = incoming (peer dialed us, not us them)
"""

import json
import sys
import time
import urllib.request

BASE = (sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8080").rstrip(
    "/"
)
CAP = int(sys.argv[2]) if len(sys.argv) > 2 else 200
BUDGET_S = 15.0
CALL_TIMEOUT_S = 2.0

API = BASE + "/api/v2"


def get(path):
    with urllib.request.urlopen(API + path, timeout=CALL_TIMEOUT_S) as r:
        return json.load(r)


def main():
    out = {
        "scrape_ok": 0,
        "torrents_active": 0,
        "torrents_scraped": 0,
        "capped": 0,
        "peers_total": 0,
        "uploading": 0,
        "interested_choked": 0,
        "downloading": 0,
        "optimistic": 0,
        "snubbed": 0,
        "upload_active": 0,
        "incoming": 0,
        "outgoing": 0,
        "peer_up_max": 0,
        "conn_bt": 0,
        "conn_utp": 0,
        "conn_web": 0,
        "up_bytes_tcp": 0,
        "up_bytes_utp": 0,
        "up_bytes_web": 0,
    }
    try:
        active = get("/torrents/info?filter=active")
        out["torrents_active"] = len(active)
        active.sort(key=lambda t: t.get("upspeed", 0), reverse=True)
        if len(active) > CAP:
            out["capped"] = 1
            active = active[:CAP]

        deadline = time.monotonic() + BUDGET_S
        for t in active:
            if time.monotonic() > deadline:
                out["capped"] = 1
                break
            h = t.get("hash")
            if not h:
                continue
            try:
                data = get("/sync/torrentPeers?hash=%s&rid=0" % h)
            except Exception:
                continue
            out["torrents_scraped"] += 1
            for p in (data.get("peers") or {}).values():
                out["peers_total"] += 1
                flags = p.get("flags", "")
                if "U" in flags:
                    out["uploading"] += 1
                if "u" in flags:
                    out["interested_choked"] += 1
                if "D" in flags:
                    out["downloading"] += 1
                if "O" in flags:
                    out["optimistic"] += 1
                if "S" in flags:
                    out["snubbed"] += 1
                if "I" in flags:
                    out["incoming"] += 1
                else:
                    out["outgoing"] += 1
                us = p.get("up_speed", 0)
                if us > 0:
                    out["upload_active"] += 1
                if us > out["peer_up_max"]:
                    out["peer_up_max"] = us
                conn = (p.get("connection") or "").lower()
                if "web" in conn:
                    out["conn_web"] += 1
                    out["up_bytes_web"] += us
                elif "utp" in conn or "μtp" in conn:
                    out["conn_utp"] += 1
                    out["up_bytes_utp"] += us
                else:
                    out["conn_bt"] += 1
                    out["up_bytes_tcp"] += us
        out["scrape_ok"] = 1
    except Exception:
        out["scrape_ok"] = 0

    sys.stdout.write(json.dumps(out) + "\n")


if __name__ == "__main__":
    main()
