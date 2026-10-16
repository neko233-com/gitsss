#!/usr/bin/env python3
"""Verify organization task isolation and avatar uploads using disposable accounts."""

import argparse
import base64
import datetime
import http.cookiejar
import json
from pathlib import Path
import re
import secrets
import struct
import urllib.error
import urllib.parse
import urllib.request
import zlib


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    args = parser.parse_args()
    base = args.url.rstrip("/")
    token = args.token_file.read_text().strip()
    suffix = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%d%H%M%S")
    orgs = ["board-probe-" + suffix, "board-scope-" + suffix]
    users = []
    created_orgs = []
    created_repos = []
    checks = []
    cleanup_errors = []
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

    def api(path, method="GET", data=None, auth=None):
        body = None if data is None else json.dumps(data).encode()
        request = urllib.request.Request(base + "/api/v1" + path, body, method=method,
                                         headers={"Authorization": auth or "token " + token,
                                                  "Content-Type": "application/json"})
        with opener.open(request, timeout=30) as response:
            data = response.read()
            return json.loads(data) if data else None

    def session(name, password):
        client = urllib.request.build_opener(urllib.request.ProxyHandler({}),
                                             urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
        client.open(base + "/user/login", timeout=20).close()
        request = urllib.request.Request(base + "/user/login",
            urllib.parse.urlencode({"user_name": name, "password": password}).encode())
        with client.open(request, timeout=30) as response:
            if "/user/login" in response.url:
                raise RuntimeError("fixture login failed")
        return client

    def web(client, path, data=None, headers=None):
        body = None if data is None else urllib.parse.urlencode(data).encode()
        request = urllib.request.Request(base + path, body, headers=headers or {})
        with client.open(request, timeout=30) as response:
            markup = response.read().decode()
            if "500 Internal Server Error" in markup or "Render failed" in markup:
                raise RuntimeError("page rendering failed: " + path)
            return markup

    def denied(client, path, data=None, headers=None, status=404):
        try:
            web(client, path, data, headers)
        except urllib.error.HTTPError as exc:
            assert exc.code == status, (exc.code, status)
        else:
            raise AssertionError("denied operation succeeded: " + path)

    def cards(markup):
        return re.findall(r'data-board-task="(\d+)"', markup)

    def passed(label):
        checks.append({"check": label, "status": "pass"})

    try:
        for org in orgs:
            api("/orgs", "POST", {"username": org, "visibility": "private"})
            created_orgs.append(org)
        teams = []
        for org in orgs:
            teams.append(api("/orgs/" + org + "/teams", "POST",
                             {"name": "contributors", "permission": "read", "units": ["repo.code"]})["id"])
        credentials = []
        for index in range(3):
            name, password = "board-user-" + str(index) + "-" + suffix, secrets.token_urlsafe(24)
            api("/admin/users", "POST", {"username": name, "password": password,
                "email": name + "@example.invalid", "must_change_password": False, "send_notify": False})
            users.append(name)
            credentials.append((name, password))
        for team in teams:
            for name in users[:2]:
                api("/teams/" + str(team) + "/members/" + name, "PUT")
        member, coworker, outsider = [session(*credential) for credential in credentials]
        board = "/" + orgs[0] + "/-/kanban"
        other = "/" + orgs[1] + "/-/kanban"
        denied(outsider, board)
        passed("shared board hidden from non-members")
        markup = web(member, board)
        assert "data-org-kanban" in markup and not cards(markup)
        markup = web(member, board + "/tasks", {"title": "Quick task @" + users[1], "status": "todo"})
        task_id = cards(markup)[0]
        assignee_id = api("/users/" + users[1])["id"]
        assert 'data-assignee="' + str(assignee_id) + '"' in markup
        passed("quick standalone card and @member assignment")
        assert task_id in cards(web(member, board + "?q=%40" + users[1]))
        assert not cards(web(member, board + "?q=missing-card"))
        passed("stored task and @assignee searches")
        web(coworker, board + "/tasks/" + task_id + "/move", {"status": "doing"})
        markup = web(member, board)
        assert re.search(r'<option value="doing" selected', markup)
        passed("organization members collaborate and moves persist")
        fragment = web(member, board + "/tasks/" + task_id,
                       {"title": "Updated task", "content": "Inline details", "status": "doing", "assignee": users[1]},
                       headers={"X-Requested-With": "XMLHttpRequest"})
        assert cards(fragment) == [task_id] and "Inline details" in fragment and "<html" not in fragment
        passed("inline card edit returns a fragment without page navigation")
        for action, data in [("", {"title": "Wrong scope", "status": "done"}),
                             ("/move", {"status": "done"}), ("/delete", {})]:
            denied(member, other + "/tasks/" + task_id + action, data)
        passed("cross-organization edit move and delete rejected")
        denied(coworker, board + "/tasks/" + task_id + "/delete", {})
        passed("non-creator member cannot delete another member card")
        denied(member, board + "/tasks/" + task_id + "/move", {"status": "invalid"}, status=400)
        web(member, board + "/tasks", {"title": "Invalid assignee @" + users[2], "status": "todo"})
        assert cards(web(member, board)) == [task_id]
        passed("invalid state and outside assignee rejected")
        denied(member, board + "/tasks", {"title": "Cross origin", "status": "todo"},
               headers={"Origin": "https://example.invalid", "Sec-Fetch-Site": "cross-site"}, status=403)
        passed("cross-origin task mutations rejected")

        def link_revision(markup, scope="personal"):
            match = re.search(r'name="scope" value="' + scope + r'"[^>]*>\s*<input[^>]*name="revision" value="([a-f0-9]+)"', markup)
            assert match, "homepage editor revision missing"
            return match[1]
        home = web(member, "/?tab=links")
        revision = link_revision(home)
        payload = [("scope", "personal"), ("revision", revision),
                   ("title", "CI fixture"), ("url", "http://ci.example.invalid:8080"),
                   ("title", "DB fixture"), ("url", "https://db.example.invalid/")]
        fragment = web(member, "/home-links", payload)
        assert "CI fixture" in fragment and "DB fixture" in fragment and "<html" not in fragment
        assert "CI fixture" in web(member, "/?tab=links") and "CI fixture" not in web(coworker, "/?tab=links")
        denied(member, "/home-links", payload, status=409)
        denied(member, "/home-links", {"scope": "shared", "revision": revision}, status=403)
        denied(member, "/home-links", {"scope": "personal", "revision": link_revision(fragment),
               "title": "Bad", "url": "javascript:alert(1)"}, status=400)
        denied(member, "/home-links", {"scope": "personal", "revision": link_revision(fragment)},
               headers={"Origin": "https://example.invalid", "Sec-Fetch-Site": "cross-site"}, status=403)
        web(member, "/home-links", {"scope": "personal", "revision": link_revision(fragment)})
        passed("multiple personal shortcuts persist with user isolation, shared authority, URL validation, stale-save and CSRF protection")
        for index in range(24):
            with opener.open(base + "/-/builtin-avatar/" + str(index), timeout=10) as response:
                assert response.read(8) == b"\x89PNG\r\n\x1a\n"
        web(member, "/user/settings/avatar", {"preset": "23"})
        basic = "Basic " + base64.b64encode((users[0] + ":" + credentials[0][1]).encode()).decode()
        preset_url = api("/user", auth=basic)["avatar_url"]
        with opener.open(preset_url, timeout=10) as response:
            assert len(response.read()) > 100
        passed("all 24 bundled avatars and authenticated selection")

        # A small compressed but non-square PNG must still be normalized by the server.
        def chunk(kind, data):
            return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
        raw = (b"\x00" + b"\x36\x91\xbf" * 900) * 300
        image = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 900, 300, 8, 2, 0, 0, 0)) + chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b"")
        boundary = secrets.token_hex(16)
        body = ("--" + boundary + '\r\nContent-Disposition: form-data; name="source"\r\n\r\nlocal\r\n--' + boundary + '\r\nContent-Disposition: form-data; name="avatar"; filename="wide.png"\r\nContent-Type: image/png\r\n\r\n').encode() + image + ("\r\n--" + boundary + "--\r\n").encode()
        request = urllib.request.Request(base + "/user/settings/avatar", body,
            headers={"Content-Type": "multipart/form-data; boundary=" + boundary})
        member.open(request, timeout=30).close()
        uploaded_url = api("/user", auth=basic)["avatar_url"]
        assert uploaded_url != preset_url
        with opener.open(uploaded_url, timeout=10) as response:
            output = response.read()
        assert output[:8] == b"\x89PNG\r\n\x1a\n"
        width, height = struct.unpack(">II", output[16:24])
        assert width == height and width <= 512 and width <= 300
        passed("real multipart upload automatically crops resizes and compresses")
        web(member, board + "/tasks/" + task_id + "/delete", {})
        assert not cards(web(member, board))
        passed("creator deletion and empty board")

        rule_path = "/org/" + orgs[0] + "/settings/team-rules"
        denied(coworker, rule_path)
        owners = next(team for team in api("/orgs/" + orgs[0] + "/teams") if team["name"] == "Owners")
        api("/teams/" + str(owners["id"]) + "/members/" + users[0], "PUT")

        def rule_revision(markup):
            return re.search(r'name="revision" value="(-?\d+)"', markup)[1]

        revision = rule_revision(web(member, rule_path))
        for name in ["server-existing", "client-existing"]:
            api("/orgs/" + orgs[0] + "/repos", "POST", {"name": name, "private": True})
            created_repos.append(orgs[0] + "/" + name)
        payload = {"action": "preview", "revision": revision, "pattern": "^server-", "team_id": teams[0], "enabled": "true"}
        preview = web(member, rule_path, payload)
        assert "server-existing" in preview and "client-existing" not in preview
        payload["action"] = "save"
        saved = web(member, rule_path, payload)
        rule_id = re.search(r'team-rules\?edit=([a-zA-Z0-9]+)', saved)[1]
        denied(member, rule_path, payload, status=409)
        for name in ["server-new", "client-new"]:
            api("/orgs/" + orgs[0] + "/repos", "POST", {"name": name, "private": True})
            created_repos.append(orgs[0] + "/" + name)

        def assigned(name):
            return teams[0] in [team["id"] for team in api("/repos/" + orgs[0] + "/" + name + "/teams")]

        assert assigned("server-new") and not assigned("client-new") and not assigned("server-existing")
        revision = rule_revision(saved)
        apply = {"action": "apply", "revision": revision, "id": rule_id}
        web(member, rule_path, apply)
        web(member, rule_path, apply)
        assert assigned("server-existing") and not assigned("client-existing")
        invalid = {"action": "save", "revision": revision, "id": rule_id, "pattern": "[", "team_id": teams[0], "enabled": "true"}
        denied(member, rule_path, invalid, status=400)
        invalid.update(pattern=".*", team_id=teams[1])
        denied(member, rule_path, invalid, status=400)
        denied(member, rule_path, apply, headers={"Origin": "https://example.invalid", "Sec-Fetch-Site": "cross-site"}, status=403)
        disabled = web(member, rule_path, {"action": "save", "revision": revision, "id": rule_id, "pattern": "^server-", "team_id": teams[0]})
        denied(member, rule_path, {"action": "apply", "revision": rule_revision(disabled), "id": rule_id}, status=400)
        web(member, rule_path, {"action": "delete", "revision": rule_revision(disabled), "id": rule_id})
        assert assigned("server-new") and assigned("server-existing")
        passed("owner-only regex rules: preview, creation, existing idempotent grants, stale editor, team scope, invalid regex, CSRF, disable/delete preserve access")
        result = {"status": "pass", "checks": checks}
    except Exception as exc:
        result = {"status": "fail", "checks": checks, "error": type(exc).__name__ + ": " + str(exc)}
    finally:
        for repo in reversed(created_repos):
            try:
                api("/repos/" + repo, "DELETE")
            except Exception:
                cleanup_errors.append("repository " + repo)
        for org in reversed(created_orgs):
            try:
                api("/orgs/" + org, "DELETE")
            except Exception:
                cleanup_errors.append("organization " + org)
        for name in reversed(users):
            try:
                api("/admin/users/" + name, "DELETE")
            except Exception:
                cleanup_errors.append("user " + name)
    result["cleanup"] = "pass" if not cleanup_errors else cleanup_errors
    if cleanup_errors:
        result["status"] = "fail"
    print(json.dumps(result, indent=2))
    return 0 if result["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
