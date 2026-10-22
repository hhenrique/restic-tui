#!/usr/bin/env python3
"""Emit restic backup status as one JSON object for the Omarchy panel.

usage: status.py [--days N] [--stale-days N] [--env-file PATH] [--cache-only]

Opening a restic repository costs a KDF derivation plus a network round trip,
so the result is cached in $XDG_CACHE_HOME/restic-backup-plugin/. The panel
asks for --cache-only when it opens (instant) and for a live run on its timer.
Stamp files written by restic-backup.service (see scripts/backup-due-check)
supply last success/failure without touching the repository at all.
"""
import argparse
import datetime as dt
import fcntl
import json
import os
import subprocess
import sys
import tempfile
from pathlib import Path

CACHE_DIR = Path(os.environ.get("XDG_CACHE_HOME", Path.home() / ".cache")) / "restic-backup-plugin"
STATE_DIR = Path(os.environ.get("XDG_STATE_HOME", Path.home() / ".local/state")) / "restic-backup"
RESTIC_TIMEOUT = 120


def load_env(path):
    """Merge `export KEY=VALUE` / `KEY=VALUE` lines into a copy of os.environ."""
    env = dict(os.environ)
    if env.get("RESTIC_REPOSITORY") and (env.get("RESTIC_PASSWORD") or env.get("RESTIC_PASSWORD_FILE")):
        return env
    p = Path(path).expanduser() if path else Path.home() / ".restic-env"
    try:
        lines = p.read_text().splitlines()
    except OSError:
        return env
    for line in lines:
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        if line.startswith("export "):
            line = line[7:].lstrip()
        key, sep, value = line.partition("=")
        if not sep:
            continue
        value = value.strip()
        if len(value) >= 2 and value[0] == value[-1] and value[0] in "'\"":
            value = value[1:-1]
        env.setdefault(key.strip(), value)
    return env


def stamp(name):
    try:
        return int((STATE_DIR / name).stat().st_mtime)
    except OSError:
        return None


def read_cache():
    try:
        return json.loads((CACHE_DIR / "snapshots.json").read_text())
    except (OSError, ValueError):
        return None


def write_cache(snaps):
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    fd, tmp = tempfile.mkstemp(dir=CACHE_DIR, prefix=".snapshots.")
    try:
        with os.fdopen(fd, "w") as handle:
            json.dump({"fetched": int(dt.datetime.now().timestamp()), "snapshots": snaps}, handle)
        os.replace(tmp, CACHE_DIR / "snapshots.json")
    except BaseException:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        raise


def fetch(env):
    """Run restic; returns (snapshots, error). Serialised by a lock so two
    shells (or a manual refresh racing the timer) never run restic twice."""
    CACHE_DIR.mkdir(parents=True, exist_ok=True)
    with open(CACHE_DIR / "lock", "w") as lock:
        try:
            fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        except OSError:
            return None, "refresh already running"
        try:
            proc = subprocess.run(["restic", "snapshots", "--json"], env=env, capture_output=True,
                                  text=True, timeout=RESTIC_TIMEOUT)
        except FileNotFoundError:
            return None, "restic is not installed"
        except subprocess.TimeoutExpired:
            return None, "restic timed out"
        if proc.returncode != 0:
            msg = (proc.stderr or proc.stdout or "restic failed").strip().splitlines()
            return None, (msg[-1] if msg else "restic failed")[:200]
        try:
            snaps = json.loads(proc.stdout or "[]")
        except ValueError:
            return None, "could not parse restic output"
        write_cache(snaps)
        return snaps, None


def parse_time(text):
    # restic emits RFC3339 with nanoseconds ("2026-10-21T09:17:03.123456789-03:00");
    # fromisoformat accepts at most 6 fractional digits.
    text = str(text)
    head, tz = text, ""
    for sign in ("+", "-"):
        i = text.rfind(sign)
        if i > 10:
            head, tz = text[:i], text[i:]
    if text.endswith("Z"):
        head, tz = text[:-1], "+00:00"
    if "." in head:
        base, frac = head.split(".", 1)
        head = base + "." + frac[:6]
    return dt.datetime.fromisoformat(head + tz).astimezone()


def summarize(snaps, days, stale_days):
    parsed = []
    for s in snaps or []:
        try:
            when = parse_time(s["time"])
        except (KeyError, ValueError):
            continue
        summary = s.get("summary") or {}
        parsed.append({
            "id": str(s.get("short_id") or str(s.get("id", ""))[:8]),
            "epoch": int(when.timestamp()),
            "date": when.strftime("%Y-%m-%d"),
            "hostname": s.get("hostname", ""),
            "paths": s.get("paths") or [],
            "dataAdded": int(summary.get("data_added") or 0),
            "totalBytes": int(summary.get("total_bytes_processed") or 0),
            "filesNew": int(summary.get("files_new") or 0),
            "filesChanged": int(summary.get("files_changed") or 0),
            "hasSummary": bool(summary),
        })
    parsed.sort(key=lambda s: s["epoch"])

    today = dt.datetime.now().astimezone().date()
    by_day = {}
    for s in parsed:
        d = by_day.setdefault(s["date"], {"count": 0, "dataAdded": 0})
        d["count"] += 1
        d["dataAdded"] += s["dataAdded"]
    grid = []
    for back in range(days - 1, -1, -1):
        day = today - dt.timedelta(days=back)
        key = day.isoformat()
        d = by_day.get(key, {"count": 0, "dataAdded": 0})
        grid.append({"date": key, "weekday": day.strftime("%a"), "count": d["count"], "dataAdded": d["dataAdded"]})

    last = parsed[-1] if parsed else None
    return {
        "snapshotCount": len(parsed),
        "last": last,
        "days": grid,
        "staleAfterDays": stale_days,
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--days", type=int, default=7)
    ap.add_argument("--stale-days", type=int, default=3)
    ap.add_argument("--env-file", default="")
    ap.add_argument("--cache-only", action="store_true")
    args = ap.parse_args()

    error = None
    cached = read_cache()
    snaps = cached["snapshots"] if cached else None
    fetched = cached["fetched"] if cached else None

    if not args.cache_only:
        fresh, error = fetch(load_env(args.env_file))
        if fresh is not None:
            snaps, fetched = fresh, int(dt.datetime.now().timestamp())

    out = summarize(snaps, max(1, args.days), args.stale_days)
    out.update({
        "ok": True,
        "hasData": snaps is not None,
        "fetched": fetched,
        "error": error,
        "lastSuccess": stamp("last-success"),
        "lastFailure": stamp("last-failure"),
    })
    json.dump(out, sys.stdout)


if __name__ == "__main__":
    main()
