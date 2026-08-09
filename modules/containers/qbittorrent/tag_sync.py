#!/usr/bin/env python3
"""Mirror each torrent's category into a matching tag (Flood shows tags, not categories)."""
import argparse
import json
import urllib.parse
import urllib.request


def api_get(api, path):
    with urllib.request.urlopen(f"{api}{path}", timeout=10) as r:
        return json.load(r)


def api_post(api, path, fields):
    data = urllib.parse.urlencode(fields).encode()
    req = urllib.request.Request(f"{api}{path}", data=data, method="POST")
    with urllib.request.urlopen(req, timeout=10) as r:
        return r.read().decode()


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--api", required=True)
    args = p.parse_args()
    for t in api_get(args.api, "/api/v2/torrents/info"):
        cat = t.get("category", "")
        if not cat:
            continue
        tags = [x.strip() for x in (t.get("tags", "") or "").split(",") if x.strip()]
        if cat in tags:
            continue
        api_post(
            args.api,
            "/api/v2/torrents/addTags",
            {"hashes": t["hash"], "tags": cat},
        )
        print(f"tagsync: {t['hash'][:8]} += tag {cat!r}")


if __name__ == "__main__":
    main()
