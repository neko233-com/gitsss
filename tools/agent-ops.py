#!/usr/bin/env python3
"""Agent operations for GitSSS. Reads by default; dispatch/rerun require --apply."""

import argparse
import json
from pathlib import Path
import sys
from urllib.parse import quote
import urllib.error
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("command", choices=["status", "runs", "jobs", "dispatch", "rerun"])
    parser.add_argument("--url", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--repo", help="owner/repository")
    parser.add_argument("--workflow", help="workflow file name from the workflows API")
    parser.add_argument("--ref", default="main")
    parser.add_argument("--run", type=int)
    parser.add_argument("--inputs-file", type=Path, help="JSON workflow inputs; do not store secrets in the repository")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    base = args.url.rstrip("/")
    token = args.token_file.read_text().strip()
    client = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())

    def request(path, method="GET", data=None):
        body = None if data is None else json.dumps(data).encode()
        req = urllib.request.Request(base + path, body, method=method,
                                     headers={"Authorization": "token " + token, "Content-Type": "application/json"})
        with client.open(req, timeout=30) as response:
            raw = response.read()
            return json.loads(raw) if raw else {"http_status": response.status}

    def repository_path():
        if not args.repo or len(args.repo.split("/")) != 2:
            parser.error("--repo owner/repository is required")
        return "/api/v1/repos/" + "/".join(quote(part, safe="") for part in args.repo.split("/"))

    try:
        if args.command == "status":
            health = request("/api/healthz")
            user = request("/api/v1/user")
            if not user.get("is_admin"):
                raise RuntimeError("site-admin token required for instance status")
            runners = request("/api/v1/admin/actions/runners?limit=100")
            queued = request("/api/v1/admin/actions/jobs?status=queued&limit=1")
            running = request("/api/v1/admin/actions/jobs?status=in_progress&limit=1")
            result = {"health": health["status"], "actor": user["login"], "runners": runners,
                      "queued_jobs": queued.get("total_count"), "running_jobs": running.get("total_count")}
        elif args.command == "runs":
            path = repository_path() + "/actions/runs" if args.repo else "/api/v1/admin/actions/runs"
            result = request(path + "?limit=20")
        elif args.command == "jobs":
            if not args.run:
                parser.error("--run is required")
            result = request(repository_path() + "/actions/runs/" + str(args.run) + "/jobs")
        else:
            if not args.apply:
                parser.error("dispatch and rerun require --apply after reviewing the workflow and target")
            path = repository_path()
            if args.command == "dispatch":
                if not args.workflow:
                    parser.error("--workflow is required")
                payload = {"ref": args.ref, "inputs": json.loads(args.inputs_file.read_text()) if args.inputs_file else {}}
                result = request(path + "/actions/workflows/" + quote(args.workflow, safe="") + "/dispatches", "POST", payload)
            else:
                if not args.run:
                    parser.error("--run is required")
                result = request(path + "/actions/runs/" + str(args.run) + "/rerun", "POST")
        print(json.dumps(result, indent=2))
    except urllib.error.HTTPError as exc:
        print(json.dumps({"error": "GitSSS API request failed", "http_status": exc.code}), file=sys.stderr)
        return 1
    except Exception as exc:
        print(json.dumps({"error": str(exc).replace(token, "[redacted]")}), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
