#!/usr/bin/env python3
"""Plays one hand through the public API: host creates, guest asks to join, host approves,
deals, and whoever acts first folds. Usage: smoke.py [base_url] [host_header]"""
import json, sys, urllib.request

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://localhost:8080"
HOST = sys.argv[2] if len(sys.argv) > 2 else None


def call(method, path, body=None):
    req = urllib.request.Request(BASE + path, method=method, data=json.dumps(body).encode() if body else None)
    if HOST:
        req.add_header("Host", HOST)
    return urllib.request.urlopen(req, timeout=5)


def post(path, body):
    with call("POST", path, body) as r:
        return json.loads(r.read() or "null")


def view(room, token):
    # The first event on the stream is the current table.
    with call("GET", f"/api/rooms/{room}/events?token={token}") as r:
        for line in r:
            if line.startswith(b"data: "):
                return json.loads(line[6:])


host = post("/api/rooms", {"name": "host"})
room = host["room"]
guest = post(f"/api/rooms/{room}/join", {"name": "guest"})
assert view(room, guest["token"])["status"] == "pending", "guest waits for approval"

req = view(room, host["token"])["requests"][0]
post(f"/api/rooms/{room}/requests/{req['id']}", {"token": host["token"], "approve": True})
assert view(room, guest["token"])["status"] == "seated", "guest seated after approval"

post(f"/api/rooms/{room}/start", {"token": host["token"]})
v = view(room, host["token"])
assert v["street"] == "preflop" and len(v["hole"]) == 2, "hand dealt"

first = host if v["toAct"] == v["you"] else guest
post(f"/api/rooms/{room}/action", {"token": first["token"], "action": "fold"})
v = view(room, host["token"])
assert v["street"] == "waiting" and v["result"]["winners"], "fold ends the hand"
chips = sum(p["chips"] for p in v["players"])
assert chips == 2 * v["startChips"], f"chips conserved, got {chips}"
print(f"ok: room {room} played a hand, winner {v['result']['winners'][0]['playerId']}")
