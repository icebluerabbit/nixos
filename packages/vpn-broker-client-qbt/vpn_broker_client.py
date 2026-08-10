"""
qBittorrent VPN broker client — replaces vpn_select / conf_patch / net_config.

The vpn-gw broker now owns tunnel selection, health, port-forward leasing,
and the inbound DNAT. This client is dumb: it holds one claim, keeps it
alive, and reconciles qbt's torrent session to the claimed tunnel's veth +
the forwarded port / announce IP the broker hands back. All
failover/selection logic lives in the broker (vpn_select dissolved).

Two modes:
  prestart  — qbittorrent ExecStartPre, before the WebUI is up. Block on
              the broker's /ready, claim + dnat a tunnel, and patch the conf
              so qbt's FIRST announce already binds the live tunnel +
              correct port/ip. Fail-closed: on any failure leave the seed
              binding (a veth; a down veth = no leak) and let keepalive
              heal.
  keepalive — timer (< broker TTL). claim (idempotent per source IP) + dnat
              (refreshes the claim, re-leases, returns the current port/ip),
              then reconcile qbt's bound interface + pin
              listen_port/announce_ip via the WebUI API. Self-heals every
              tick: a dead tunnel is voided broker-side, so the next claim()
              returns another healthy one.

Config JSON (path in argv[2]):
  { "broker": "http://192.168.5.9:9099", "api": "http://127.0.0.1:8080",
    "conf": "/var/lib/qbittorrent/qBittorrent/config/qBittorrent.conf",
    "client_name": "qbittorrent", "pool": {"wg1": "vpn1", "wg2": "vpn2"},
    "state_dir": "/var/lib/qbittorrent/.vpnbroker" }
"""

import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

mode = sys.argv[1]
cfg = json.load(open(sys.argv[2]))
BROKER = cfg["broker"].rstrip("/")
API = cfg["api"].rstrip("/")
CONF = cfg["conf"]
CLIENT = cfg.get("client_name", "")
POOL = cfg[
    "pool"
]  # tunnel name -> {"veth": ..., "ip": ...} (from qbt's vpnPool)
STATE_DIR = cfg.get(
    "state_dir", ""
)  # persistent (bind-mounted) dir for cross-restart state


# ── persistent state ────────────────────────────────────────────────────
# Survives the ephemeral container via the bind-mounted profile dir.
def _state_path(name):
    return os.path.join(STATE_DIR, name) if STATE_DIR else ""


def read_pref():
    # Last tunnel we held — re-requested on claim so a broker wipe doesn't
    # rotate our exit IP.
    try:
        return open(_state_path("tunnel")).read().strip()
    except Exception:
        return ""


def save_pref(tunnel):
    p = _state_path("tunnel")
    if not p:
        return
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
        with open(p, "w") as f:
            f.write(tunnel)
    except Exception:
        pass


def _load_json(name):
    try:
        return json.load(open(_state_path(name)))
    except Exception:
        return {}


def _save_json(name, obj):
    p = _state_path(name)
    if not p:
        return
    try:
        os.makedirs(STATE_DIR, exist_ok=True)
        with open(p, "w") as f:
            json.dump(obj, f)
    except Exception:
        pass


# ── broker RPC ──────────────────────────────────────────────────────────
def broker(method, path, body=None):
    data = json.dumps(body).encode() if body is not None else None
    headers = {"Content-Type": "application/json"} if data else {}
    req = urllib.request.Request(
        BROKER + path, data=data, method=method, headers=headers
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as r:
            raw = r.read().decode()
            return r.status, (
                json.loads(raw) if raw.strip().startswith("{") else raw
            )
    except urllib.error.HTTPError as e:
        return e.code, ""
    except Exception:
        return 0, ""


def wait_ready(deadline):
    while time.time() < deadline:
        if broker("GET", "/ready")[0] == 200:
            return True
        time.sleep(2)
    return False


def claim():
    req = {"client_name": CLIENT}
    pref = read_pref()
    if pref:
        # Soft: broker grants only if healthy + free, else picks for us.
        req["prefer"] = pref
    st, body = broker("POST", "/claim", req)
    t = body.get("tunnel") if st == 200 and isinstance(body, dict) else None
    if t:
        save_pref(t)  # remember what we actually got, for the next re-converge
    return t


def dnat(tunnel):
    # Tell the broker our own bridge IP on this tunnel to DNAT inbound to
    # (trusted VLAN).
    st, body = broker("POST", "/dnat/" + tunnel, {"ip": POOL[tunnel]["ip"]})
    if st == 200 and isinstance(body, dict):
        return body.get("port"), body.get("exit_ip", "")
    return None, None


# ── WebUI API (keepalive rebind + pin) ──────────────────────────────────
def api_prefs():
    try:
        with urllib.request.urlopen(
            API + "/api/v2/app/preferences", timeout=10
        ) as r:
            return json.load(r)
    except Exception:
        return {}


def api_set(prefs):
    data = urllib.parse.urlencode({"json": json.dumps(prefs)}).encode()
    urllib.request.urlopen(
        urllib.request.Request(API + "/api/v2/app/setPreferences", data=data),
        timeout=10,
    )


# ── conf patch (prestart, before the WebUI is up) ───────────────────────
def patch_conf(veth, port, exit_ip):
    if not os.path.exists(CONF):
        return
    with open(CONF) as f:
        lines = f.readlines()
    out, in_bt = [], False
    drop = (
        "Session\\Port=",
        "Session\\AnnounceIP=",
        "Session\\Interface=",
        "Session\\InterfaceName=",
    )
    for line in lines:
        s = line.rstrip("\n")
        if s == "[BitTorrent]":
            out.append(line)
            out.append(f"Session\\Port={port}\n")
            out.append(f"Session\\Interface={veth}\n")
            out.append(f"Session\\InterfaceName={veth}\n")
            if exit_ip:
                out.append(f"Session\\AnnounceIP={exit_ip}\n")
            in_bt = True
            continue
        if s.startswith("["):
            in_bt = False
        if in_bt and s.startswith(drop):
            continue  # drop stale copies → idempotent on re-run
        out.append(line)
    with open(CONF, "w") as f:
        f.writelines(out)


# ── MAM dynamic seedbox ─────────────────────────────────────────────────
# Re-lock the tracker session to the current VPN exit IP.
#
# Private trackers authorize a seedbox by a single stored IP; on failover our
# exit IP changes and announces get rejected until MAM is re-pointed. We POST
# the current IP from the claimed tunnel (curl --interface <veth> so MAM sees
# the VPN exit, not eth0). Fires on an IP change (re-lock, capped to ~1/hr by
# floor_seconds since MAM rate-limits CHANGES → sooner just 429s) AND on a
# refresh interval when the IP is unchanged: MAM rotates mam_id on every
# request and the session ages out if never exercised, so a periodic "No
# Change" call keeps it alive (announces need a valid session to re-lock on
# the next failover). mam_id is a cookie jar (curl -c) so the rotated cookie
# persists; seeded once from the sops secret.
def mam_update(veth, exit_ip):
    mam = cfg.get("mam") or {}
    if not mam.get("enable") or not exit_ip:
        return
    jar = _state_path("mam.cookies")
    change_floor = int(
        mam.get("floor_seconds", 3600)
    )  # MAM rate-limits CHANGES to ~1/hr
    refresh = int(
        mam.get("refresh_seconds", 3600)
    )  # keepalive cadence (sessions age out)
    st = _load_json("mam.json")
    now = time.time()
    changed = st.get("ip") != exit_ip
    if changed:
        # New exit IP → re-lock, but not more than once per change_floor
        # (else 429). Record the attempt (not just success) so a persistent
        # 429/403 backs off to 1/floor, not every tick.
        if now - st.get("change_ts", 0) < change_floor:
            return
        st["change_ts"] = now
    elif now - st.get("ok_ts", 0) < refresh:
        return  # same IP and session still fresh — nothing to do
    # else: same IP, refresh interval elapsed → keepalive ("No Change") so
    # MAM rotates the cookie and the session stays valid for the next
    # failover re-lock.
    _save_json("mam.json", st)
    if os.path.exists(jar):
        cookie = ["-b", jar]
    else:
        try:
            cookie = ["-b", "mam_id=" + open(mam["seed_path"]).read().strip()]
        except Exception as e:
            print(
                f"qbt-mam: cannot read seed {mam.get('seed_path')!r} ({e})",
                flush=True,
            )
            return
    try:
        r = subprocess.run(
            [
                "curl",
                "-sS",
                "--interface",
                veth,
                "--max-time",
                "15",
                "-c",
                jar,
                *cookie,
                mam["endpoint"],
            ],
            capture_output=True,
            text=True,
            timeout=20,
        )
    except Exception as e:
        print(
            f"qbt-mam: curl failed ({e}); retry next eligible tick", flush=True
        )
        return
    try:
        j = json.loads(r.stdout.strip())
    except Exception:
        print(
            f"qbt-mam: non-JSON reply (rc={r.returncode}): "
            f"{r.stdout.strip()[:200]!r}",
            flush=True,
        )
        return
    msg = j.get("msg", "")
    if j.get("Success"):
        st["ip"] = exit_ip
        st["ok_ts"] = (
            now  # marks the session refreshed; drives the keepalive cadence
        )
        _save_json("mam.json", st)
        print(f"qbt-mam: seedbox IP {exit_ip} via {veth} ({msg})", flush=True)
    elif "too recent" in msg.lower():
        print(f"qbt-mam: rate-limited ({msg}); retry after floor", flush=True)
    else:
        # ASN mismatch / invalid session / wrong session type → operator
        # must fix the MAM config.
        print(
            f"qbt-mam: FAILED ({msg}) — "
            "check MAM session ASN allowlist + type",
            flush=True,
        )


def do_prestart():
    # Block until the broker has a live tunnel, so the first announce binds
    # correctly.
    if not wait_ready(time.time() + 60):
        print(
            "qbt-vpn: broker not ready, fail-closed (seed binding kept)",
            flush=True,
        )
        return
    t = claim()
    if not t:
        print("qbt-vpn: no tunnel from broker, fail-closed", flush=True)
        return
    port, exit_ip = dnat(t)
    if not port:
        print("qbt-vpn: dnat failed, fail-closed", flush=True)
        return
    veth = POOL[t]["veth"]
    patch_conf(veth, port, exit_ip)
    print(
        f"qbt-vpn: prestart tunnel={t} veth={veth} "
        f"port={port} announce={exit_ip}",
        flush=True,
    )


def do_keepalive():
    t = (
        claim()
    )  # idempotent: our current claim, or a fresh one if the old was voided
    if not t:
        print(
            "qbt-vpn: no tunnel available (killswitch: keep current binding)",
            flush=True,
        )
        return
    port, exit_ip = dnat(t)  # sets DNAT + re-leases + refreshes the claim TTL
    if not port:
        print(f"qbt-vpn: dnat failed on {t}; re-pick next tick", flush=True)
        return
    veth = POOL[t]["veth"]
    mam_update(
        veth, exit_ip
    )  # re-lock the MAM session if our exit IP changed (independent of WebUI)
    # The broker lease is renewed above (the critical part). The API pin is
    # best-effort: if the WebUI isn't up yet (boot/restart) api_prefs()
    # returns {} → skip and re-pin next tick, so a transiently-down WebUI
    # never fails this unit.
    prefs = api_prefs()
    if not prefs:
        print(
            f"qbt-vpn: keepalive tunnel={t} lease renewed; "
            "WebUI not ready, pin deferred",
            flush=True,
        )
        return
    try:
        if prefs.get("current_network_interface") != veth:
            api_set(
                {
                    "current_network_interface": veth,
                    "current_interface_address": "",
                }
            )
        api_set(
            {
                "announce_ip": exit_ip,
                "listen_port": int(port),
                "random_port": False,
                "reannounce_when_address_changed": True,
            }
        )
    except Exception as e:
        print(
            f"qbt-vpn: keepalive API pin failed ({e}); retry next tick",
            flush=True,
        )
        return
    print(
        f"qbt-vpn: keepalive tunnel={t} veth={veth} "
        f"port={port} announce={exit_ip}",
        flush=True,
    )


if __name__ == "__main__":
    (do_prestart if mode == "prestart" else do_keepalive)()
