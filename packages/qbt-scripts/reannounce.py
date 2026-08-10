"""Force re-announce for not-yet-started torrents whose trackers all failed."""

import argparse
import json
import urllib.parse
import urllib.request


def api_get(api, path, params=None):
    url = f"{api}{path}"
    if params:
        url += "?" + urllib.parse.urlencode(params)
    with urllib.request.urlopen(url, timeout=10) as r:
        return json.load(r)


def api_post(api, path, fields):
    data = urllib.parse.urlencode(fields).encode()
    req = urllib.request.Request(f"{api}{path}", data=data, method="POST")
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.read().decode()


def all_trackers_failed(trackers):
    # Exclude DHT/PeX/LSD pseudo-entries — they're not real HTTP trackers.
    real = [t for t in trackers if not t["url"].startswith("** [")]
    if not real:
        return False
    # 4=NotWorking (HTTP error), 5=TrackerError (e.g. "unregistered torrent"),
    # 6=Unreachable — all failed. 0=Disabled/1=NotContacted/2=Working/
    # 3=Updating are all ok. qBittorrent 5.x split the old catch-all 4 into
    # 4/5/6, so `== 4` misses 5 and 6.
    return all(t["status"] >= 4 for t in real)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--api", required=True)
    args = p.parse_args()

    torrents = api_get(args.api, "/api/v2/torrents/info")
    for t in torrents:
        if t.get("progress", 0.0) != 0.0:
            continue
        h = t["hash"]
        trackers = api_get(args.api, "/api/v2/torrents/trackers", {"hash": h})
        if all_trackers_failed(trackers):
            api_post(args.api, "/api/v2/torrents/reannounce", {"hashes": h})
            print(
                f"reannounce: {h[:8]} ({t.get('name', '')[:50]}) "
                "all trackers errored, reannouncing"
            )


if __name__ == "__main__":
    main()
