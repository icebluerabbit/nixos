"""Import .torrent files from watch/<category>/ into qBittorrent.

Each import is tagged with the category its watch subdirectory names.

A hand-dropped torrent enters the client with nothing in front of it: no
autobrr grab, no *arr grab. A cross-seed inject looks exactly the same from
qBittorrent's side, so absence of an upstream event cannot tell the two apart.
--report-url, when given, says so explicitly at the moment of import.
"""

import argparse
import glob
import json
import os
import shutil
import sys
import urllib.request
import uuid


def add_torrent(api, torrent_path, category):
    boundary = uuid.uuid4().hex
    with open(torrent_path, "rb") as f:
        content = f.read()

    def field(name, value):
        return (
            f"--{boundary}\r\n"
            f'Content-Disposition: form-data; name="{name}"\r\n\r\n'
            f"{value}\r\n"
        ).encode()

    body = field("category", category) + field("autoTMM", "true")
    body += (
        f"--{boundary}\r\n"
        f'Content-Disposition: form-data; name="torrents"; '
        f'filename="{os.path.basename(torrent_path)}"\r\n'
        f"Content-Type: application/x-bittorrent\r\n\r\n"
    ).encode()
    body += content + b"\r\n"
    body += f"--{boundary}--\r\n".encode()

    req = urllib.request.Request(
        f"{api}/api/v2/torrents/add",
        data=body,
        method="POST",
        headers={"Content-Type": f"multipart/form-data; boundary={boundary}"},
    )
    with urllib.request.urlopen(req, timeout=15) as resp:
        raw = resp.read().decode().strip()
    if raw.lower() == "ok.":  # qBittorrent < 5.1
        return True
    try:
        return json.loads(raw).get("success_count", 0) >= 1
    except Exception:
        return False


def report(url, source, category):
    """Best-effort provenance ping. Never raises: the import already
    succeeded and a recorder being down must not turn that into a failure."""
    if not url:
        return
    body = json.dumps(
        {"event": "watch_added", "name": source, "category": category}
    ).encode()
    req = urllib.request.Request(
        url,
        data=body,
        method="POST",
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(req, timeout=10):
            pass
    except Exception as e:
        print(f"watch: report failed {source}: {e}", file=sys.stderr)


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--api", required=True)
    p.add_argument("--watch-dir", required=True)
    p.add_argument("--backup-dir", required=True)
    p.add_argument("--categories", required=True)
    p.add_argument(
        "--report-url",
        default="",
        help="POST each import here; empty disables reporting",
    )
    args = p.parse_args()

    cats = [c for c in args.categories.split(",") if c]
    os.makedirs(args.backup_dir, exist_ok=True)

    for cat in cats:
        for tf in glob.glob(os.path.join(args.watch_dir, cat, "*.torrent")):
            try:
                ok = add_torrent(args.api, tf, cat)
            except Exception as e:
                print(f"watch: add failed {tf}: {e}", file=sys.stderr)
                continue
            if not ok:
                print(f"watch: add rejected {tf}", file=sys.stderr)
                continue
            name = os.path.basename(tf)
            shutil.move(tf, os.path.join(args.backup_dir, name))
            print(f"watch: added {name} category={cat}")
            report(args.report_url, name, cat)


if __name__ == "__main__":
    main()
