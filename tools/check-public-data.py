#!/usr/bin/env python3
"""Check public source/history against a private literal deny-list; never print matches."""

import argparse
import json
from pathlib import Path
import subprocess


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--deny-file", type=Path, required=True,
                        help="Private JSON list of strings, or replacement-map keys; keep outside Git")
    parser.add_argument("--all-history", action="store_true", help="Scan all locally reachable objects")
    args = parser.parse_args()
    root = Path(subprocess.check_output(["git", "rev-parse", "--show-toplevel"], text=True).strip())
    deny_path = args.deny_file.resolve()
    if deny_path.is_relative_to(root):
        parser.error("deny-file must be outside the repository")
    values = json.loads(deny_path.read_text(encoding="utf-8"))
    values = list(values) if isinstance(values, dict) else values
    if not isinstance(values, list) or not values or any(not isinstance(s, str) or not s for s in values):
        parser.error("deny-file must contain nonempty literal strings")
    needles = [s.encode() for s in values]
    hits = []
    scanned = 0
    if args.all_history:
        objects = subprocess.check_output(["git", "rev-list", "--objects", "--all"], text=True).splitlines()
        child = subprocess.Popen(["git", "cat-file", "--batch"], stdin=subprocess.PIPE, stdout=subprocess.PIPE)
        try:
            for item in objects:
                oid = item.split()[0]
                child.stdin.write((oid + "\n").encode())
                child.stdin.flush()
                header = child.stdout.readline().decode().split()
                content = child.stdout.read(int(header[2]))
                assert child.stdout.read(1) == b"\n"
                scanned += 1
                if any(needle in content for needle in needles):
                    hits.append({"object": oid, "type": header[1]})
        finally:
            child.stdin.close()
            child.wait()
    else:
        names = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"])
        for name in sorted(set(names.decode().split("\0")) - {""}):
            file = root / name
            if not file.is_file():
                continue
            scanned += 1
            if any(needle in file.read_bytes() for needle in needles):
                hits.append({"file": name})
    print(json.dumps({"status": "fail" if hits else "pass", "scanned": scanned,
                      "scope": "reachable history" if args.all_history else "current source", "hits": hits}, indent=2))
    return int(bool(hits))


if __name__ == "__main__":
    raise SystemExit(main())
