"""
slskd VPN broker client — claims a tunnel from the vpn-gw broker and routes
slskd's Soulseek traffic through it, keeping the web UI + all homelab
traffic on the LAN.

Unlike qbittorrent (which binds its libtorrent session to a tunnel veth),
slskd/.NET has no per-app interface binding for OUTBOUND peer connections
(verified: Soulseek.NET binds only its listener to listen_ip_address;
outbound uses TcpClient.ConnectAsync with no local bind). So we steer slskd
by the ROUTING TABLE instead:

  default       -> claimed tunnel veth  (outbound Soulseek → VPN; kernel
                                         sources from the veth IP)
  10.10.0.0/16  -> LAN gateway, eth0    (web UI, broker, Loki/Prometheus
                                         stay on the LAN)
  <download /24>, <veth /24s> -> connected  (automatic)

No claim → `blackhole default` (killswitch: Soulseek drops with no leak;
the LAN carve-out keeps the web UI + broker reachable so we can recover).
Inbound peers arrive via the broker's DNAT on the claimed tunnel → slskd
listening on 0.0.0.0; the accepted socket's local addr is the veth IP so
replies source correctly.

The forwarded port is applied live via slskd's runtime API (PATCH
/api/v0/options → listenPort; "the Soulseek server is updated
automatically; no reconnect"). slskd's own auth is disabled (Authelia/LDAP
gates the UI at Caddy), so the localhost API needs no token.

Runs as ROOT (needs `ip route`). Two modes:
  prestart   — before slskd starts: set the route so slskd's very first
               outbound is already VPN'd.
  keepalive  — ~45s timer: reassert the route + PATCH listenPort to the
               current lease.

Config JSON (argv[2]):
  { "broker": "http://10.10.130.4:9099", "api": "http://localhost:5030",
    "client_name": "slskd", "lan_cidr": "10.10.0.0/16",
    "lan_gateway": "10.10.130.1", "lan_dev": "eth0",
    "ip_bin": "/nix/store/…/ip", "state_dir": "/var/lib/slskd/.vpnbroker",
    "pool": { "wg1": {"veth": "vpn1", "ip": "192.168.1.3",
                      "gateway": "192.168.1.1"}, ... } }
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
CLIENT = cfg.get("client_name", "slskd")
POOL = cfg["pool"]  # tunnel -> {veth, ip, gateway}
LAN_CIDR = cfg["lan_cidr"]
LAN_GW = cfg["lan_gateway"]
LAN_DEV = cfg["lan_dev"]
IP = cfg.get("ip_bin", "ip")
STATE_DIR = cfg.get(
    "state_dir", ""
)  # persistent (bind-mounted) dir for cross-restart state


# ── persistent state ────────────────────────────────────────────────────
# Survives the ephemeral container via the bind-mounted state dir.
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


# ── broker RPC ──────────────────────────────────────────────────────────
# Same shape as qbt's client; containers don't share files.
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
    st, body = broker("POST", "/dnat/" + tunnel, {"ip": POOL[tunnel]["ip"]})
    if st == 200 and isinstance(body, dict):
        return body.get("port"), body.get("exit_ip", "")
    return None, None


# ── routing (root) ──────────────────────────────────────────────────────
def _ip(*args):
    subprocess.run([IP, *args], check=False)


def carveout():
    # Homelab traffic stays on the LAN, never the tunnel (web UI, broker,
    # metrics).
    _ip("route", "replace", LAN_CIDR, "via", LAN_GW, "dev", LAN_DEV)


def default_via(tunnel):
    v = POOL[tunnel]
    _ip("link", "set", "dev", v["veth"], "mtu", "1420")  # tunnel path-MTU
    _ip("route", "replace", "default", "via", v["gateway"], "dev", v["veth"])


def blackhole():
    _ip("route", "replace", "blackhole", "default")


# ── slskd runtime API ───────────────────────────────────────────────────
# auth off + remote_configuration on → PATCH listenPort
def slskd_set_port(port):
    try:
        req = urllib.request.Request(
            API + "/api/v0/options",
            data=json.dumps({"soulseek": {"listenPort": int(port)}}).encode(),
            headers={"Content-Type": "application/json"},
            method="PATCH",
        )
        urllib.request.urlopen(req, timeout=10)
    except Exception as e:
        print(
            f"slskd-vpn: listenPort PATCH failed ({e}; WebUI not up yet?); "
            "retry next tick",
            flush=True,
        )


# ── modes ───────────────────────────────────────────────────────────────
def do_prestart():
    carveout()
    if not wait_ready(time.time() + 60):
        blackhole()
        print(
            "slskd-vpn: broker not ready, fail-closed (blackhole default)",
            flush=True,
        )
        return
    t = claim()
    if not t:
        blackhole()
        print(
            "slskd-vpn: no tunnel from broker, fail-closed "
            "(blackhole default)",
            flush=True,
        )
        return
    port, _ = dnat(t)
    if not port:
        blackhole()
        print(
            "slskd-vpn: dnat failed, fail-closed (blackhole default)",
            flush=True,
        )
        return
    default_via(t)
    print(
        f"slskd-vpn: prestart routed default via {POOL[t]['veth']} "
        f"(tunnel={t}, port={port})",
        flush=True,
    )


def do_keepalive():
    carveout()
    t = claim()
    if not t:
        blackhole()
        print(
            "slskd-vpn: no tunnel available (killswitch: blackhole default)",
            flush=True,
        )
        return
    port, _ = dnat(t)
    if not port:
        print(f"slskd-vpn: dnat failed on {t}; re-pick next tick", flush=True)
        return
    default_via(t)
    slskd_set_port(port)
    print(
        f"slskd-vpn: keepalive default via {POOL[t]['veth']} "
        f"(tunnel={t}) listenPort={port}",
        flush=True,
    )


if __name__ == "__main__":
    (do_prestart if mode == "prestart" else do_keepalive)()
