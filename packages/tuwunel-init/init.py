"""Declaratively provision tuwunel users, then rooms/spaces, after startup.

Usage: init.py <port> <server_name> <admin_user> <users_json> <rooms_json>

Users are created via tuwunel's Synapse-compatible shared-secret registration
API (GET nonce + HMAC-SHA1 over nonce\\0user\\0pass\\0(admin|notadmin), then
POST /_synapse/admin/v1/register) — passwords ride in the request body,
never on the server's argv. Per-user passwords come from TUWUNEL_<USER>_PASS
env vars; the shared secret from TUWUNEL_REG_SECRET. Idempotent: an
already-registered user
returns "in use" and is skipped, and benign room conflicts are ignored.
"""

import hashlib
import hmac
import json
import os
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

port, server_name, admin_user, users_json, rooms_json = sys.argv[1:6]
base = f"http://localhost:{port}"
api = f"{base}/_matrix/client/v3"
synapse = f"{base}/_synapse/admin/v1"
users = json.loads(users_json)
rooms = json.loads(rooms_json)
reg_secret = os.environ["TUWUNEL_REG_SECRET"]


def pass_for(user):
    return os.environ.get(
        "TUWUNEL_" + user.upper().replace("-", "_") + "_PASS"
    )


def req(method, url, token=None, body=None):
    data = json.dumps(body).encode() if body is not None else None
    r = urllib.request.Request(url, data=data, method=method)
    r.add_header("Content-Type", "application/json")
    if token:
        r.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(r) as resp:
            return json.loads(resp.read() or "null")
    except urllib.error.HTTPError as e:
        # Idempotent re-runs hit user-in-use / alias-in-use /
        # already-joined; all fine.
        return {"_error": e.code, "_body": e.read().decode(errors="replace")}


def upload(path, token):
    """Upload an image and return its mxc:// URI, or None."""
    try:
        with open(path, "rb") as f:
            data = f.read()
    except OSError:
        return None
    fname = enc(os.path.basename(path))
    r = urllib.request.Request(
        f"{base}/_matrix/media/v3/upload?filename={fname}",
        data=data,
        method="POST",
    )
    r.add_header("Content-Type", "image/png")
    r.add_header("Authorization", f"Bearer {token}")
    try:
        with urllib.request.urlopen(r) as resp:
            return json.loads(resp.read() or "null").get("content_uri")
    except urllib.error.HTTPError as e:
        print(
            f"init: upload failed -> {e.code} "
            f"{e.read().decode(errors='replace')}",
            file=sys.stderr,
        )
        return None


def register(username, admin):
    """Create a local user via the shared-secret HMAC register API."""
    password = pass_for(username)
    if not password:
        print(f"init: no password for {username}, skipping", file=sys.stderr)
        return
    nonce = req("GET", f"{synapse}/register").get("nonce")
    if not nonce:
        print(f"init: no register nonce, skipping {username}", file=sys.stderr)
        return
    flag = b"admin" if admin else b"notadmin"
    mac = hmac.new(reg_secret.encode(), digestmod=hashlib.sha1)
    for part in (nonce.encode(), username.encode(), password.encode()):
        mac.update(part)
        mac.update(b"\x00")
    mac.update(flag)
    res = req(
        "POST",
        f"{synapse}/register",
        body={
            "nonce": nonce,
            "username": username,
            "password": password,
            "admin": admin,
            "mac": mac.hexdigest(),
            "inhibit_login": True,
        },
    )
    err = res.get("_error")
    if err and "available" not in res.get("_body", "").lower():
        # "User ID is not available" == already exists (idempotent);
        # anything else is a real failure.
        print(
            f"init: register {username} -> {err} {res.get('_body', '')}",
            file=sys.stderr,
        )


def login(user):
    res = req(
        "POST",
        f"{api}/login",
        body={
            "type": "m.login.password",
            "identifier": {"type": "m.id.user", "user": user},
            "password": pass_for(user),
        },
    )
    return res.get("access_token")


def enc(s):
    return urllib.parse.quote(s, safe="")


def full_alias(localpart):
    return f"#{localpart}:{server_name}"


def resolve_id(localpart):
    res = req("GET", f"{api}/directory/room/{enc(full_alias(localpart))}")
    return res.get("room_id")


# Wait for the homeserver to come up.
while True:
    try:
        urllib.request.urlopen(f"{base}/_matrix/client/versions").read()
        break
    except Exception:
        time.sleep(2)

# Provision users first (admin flag from the declared list), so login()
# below works.
for u in users:
    register(u["username"], u.get("admin", False))

# Set user avatars (each user sets its own; skip if one is already set).
for u in users:
    af = u.get("avatarFile")
    if not af:
        continue
    tok = login(u["username"])
    if not tok:
        continue
    mxid = enc("@" + u["username"] + ":" + server_name)
    prof = f"{api}/profile/{mxid}/avatar_url"
    if req("GET", prof).get("avatar_url"):
        continue
    mxc = upload(af, tok)
    if mxc:
        req("PUT", prof, token=tok, body={"avatar_url": mxc})

admin_token = login(admin_user)

# Pass 1: create every room/space and populate members.
for room in rooms:
    body = {
        "name": room["name"],
        "room_alias_name": room["alias"],
        "preset": "private_chat",
    }
    if room.get("space"):
        body["creation_content"] = {"type": "m.space"}
    req("POST", f"{api}/createRoom", token=admin_token, body=body)

    rid = resolve_id(room["alias"])
    if not rid:
        continue
    # Ensure admin membership even on idempotent re-runs (createRoom only
    # auto-joins the creator on first creation).
    req("POST", f"{api}/join/{enc(rid)}", token=admin_token)

    # Room avatar (skip if one is already set).
    icf = room.get("iconFile")
    if icf and not req(
        "GET", f"{api}/rooms/{enc(rid)}/state/m.room.avatar"
    ).get("url"):
        mxc = upload(icf, admin_token)
        if mxc:
            req(
                "PUT",
                f"{api}/rooms/{enc(rid)}/state/m.room.avatar",
                token=admin_token,
                body={"url": mxc},
            )

    for member in room.get("members", []):
        req(
            "POST",
            f"{api}/rooms/{enc(rid)}/invite",
            token=admin_token,
            body={"user_id": f"@{member}:{server_name}"},
        )
        member_token = login(member)
        if member_token:
            req("POST", f"{api}/join/{enc(rid)}", token=member_token)

# Pass 2: wire space hierarchy (m.space.child on the parent, m.space.parent
# on the child).
for room in rooms:
    parent = room.get("parent", "")
    if not parent:
        continue
    child_id = resolve_id(room["alias"])
    parent_id = resolve_id(parent)
    if not child_id or not parent_id:
        continue
    req(
        "PUT",
        f"{api}/rooms/{enc(parent_id)}/state/m.space.child/{enc(child_id)}",
        token=admin_token,
        body={"via": [server_name]},
    )
    req(
        "PUT",
        f"{api}/rooms/{enc(child_id)}/state/m.space.parent/{enc(parent_id)}",
        token=admin_token,
        body={"via": [server_name], "canonical": True},
    )
