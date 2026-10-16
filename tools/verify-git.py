#!/usr/bin/env python3
"""Exercise a GitSSS instance from a real Git/LFS/SSH client; print JSON evidence."""

import argparse
import base64
import datetime
import hashlib
import io
import json
import os
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.request
import zipfile


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args):
        return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--url", required=True)
    parser.add_argument("--token-file", type=Path, required=True)
    parser.add_argument("--ssh-known-hosts", type=Path)
    parser.add_argument("--keep-fixture", action="store_true")
    parser.add_argument("--actions", action="store_true", help="Run a .gitea workflow on an isolated verification instance")
    parser.add_argument("--runner-label", default="self-hosted")
    parser.add_argument("--demo-credentials-file", type=Path,
                        help="Exclusive 0600 file for the disposable reader login; keep until screenshots are captured")
    args = parser.parse_args()
    base = args.url.rstrip("/")
    token = args.token_file.read_text().strip()
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    checks = []
    resources = {}
    suffix = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%d%H%M%S")
    owner, repo = "gitsss-verification", "git-probe-" + suffix
    repo_path = "/repos/" + owner + "/" + repo
    reader_name, reader_password = "git-probe-" + suffix, secrets.token_urlsafe(24)

    def api(path, method="GET", data=None, raw=False, auth=None):
        encoded = None if data is None else json.dumps(data).encode()
        req = urllib.request.Request(base + "/api/v1" + path, encoded, method=method)
        req.add_header("Authorization", auth or "token " + token)
        req.add_header("Content-Type", "application/json")
        try:
            with opener.open(req, timeout=60) as response:
                body = response.read()
                return body if raw else json.loads(body) if body else None
        except urllib.error.HTTPError as exc:
            raise RuntimeError(method + " " + path + ": HTTP " + str(exc.code)) from None

    def auth_env(user, password):
        header = "Authorization: Basic " + base64.b64encode((user + ":" + password).encode()).decode()
        env = os.environ.copy()
        env.update(GIT_TERMINAL_PROMPT="0", GCM_INTERACTIVE="never", GIT_CONFIG_COUNT="3",
                   GIT_CONFIG_KEY_0="http." + base + "/.extraHeader", GIT_CONFIG_VALUE_0=header,
                   GIT_CONFIG_KEY_1="credential.helper", GIT_CONFIG_VALUE_1="",
                   GIT_CONFIG_KEY_2="http.followRedirects", GIT_CONFIG_VALUE_2="false")
        return env

    def git(*params, cwd=None, env=None, fail=False):
        run = subprocess.run(["git", *params], cwd=cwd, env=env or admin_env,
                             capture_output=True, text=True, encoding="utf-8", errors="replace", timeout=120)
        if fail:
            if run.returncode == 0:
                raise RuntimeError("Git unexpectedly allowed a denied operation")
            return run.stdout
        if run.returncode:
            raise RuntimeError("Git " + params[0] + " failed: " + run.stderr)
        return run.stdout.strip()

    def passed(name):
        checks.append({"check": name, "status": "pass"})

    admin_env = None
    key_id = None
    result = None
    exit_code = 0
    try:
        admin = api("/user")["login"]
        admin_env = auth_env(admin, token)
        try:
            api("/orgs/" + owner)
        except RuntimeError as e:
            if "HTTP 404" not in str(e):
                raise
            api("/orgs", "POST", {"username": owner, "full_name": "Enterprise verification", "visibility": "private"})
        fixture = api("/orgs/" + owner + "/repos", "POST", {"name": repo, "private": True, "auto_init": False, "default_branch": "main"})
        resources["repo"] = True
        teams = api("/orgs/" + owner + "/teams")
        owners = next(t for t in teams if t["permission"] == "owner")
        renamed = api("/teams/" + str(owners["id"]), "PATCH", {"display_name": "Verification administrators"})
        assert renamed["id"] == owners["id"] and renamed["name"] == owners["name"] and renamed["permission"] == "owner"
        assert renamed["display_name"] == "Verification administrators"
        passed("owner team label preserves identity and authority")
        try:
            api("/teams/" + str(owners["id"]), "PATCH", {"name": "renamed-owner"})
            raise AssertionError("owner identity rename was accepted")
        except RuntimeError as e:
            assert "HTTP 422" in str(e)
        passed("owner identity rename rejected")
        with tempfile.TemporaryDirectory(prefix="gitsss-verify-") as tmp:
            root = Path(tmp)
            work = root / "work"
            work.mkdir()
            git("init", "--initial-branch=main", str(work))
            for key, value in [("user.name", "GitSSS verification"), ("user.email", "verify@localhost.invalid"), ("commit.gpgsign", "false"), ("tag.gpgsign", "false")]:
                git("config", key, value, cwd=work)
            git("lfs", "install", "--local", cwd=work)
            git("remote", "add", "origin", fixture["clone_url"], cwd=work)
            (work / "README.md").write_text("# Enterprise verification\n\nDisposable Git protocol fixture.\n")
            (work / ".gitattributes").write_text("*.bin filter=lfs diff=lfs merge=lfs -text\n")
            payload = bytes(range(256)) * 256
            (work / "payload.bin").write_bytes(payload)
            git("add", "README.md", ".gitattributes", "payload.bin", cwd=work)
            git("commit", "-m", "test: initialize Git and LFS verification", cwd=work)
            git("push", "-u", "origin", "main", cwd=work)
            passed("HTTP create commit and push with LFS")
            clone = root / "clone"
            git("clone", fixture["clone_url"], str(clone))
            assert hashlib.sha256((clone / "payload.bin").read_bytes()).digest() == hashlib.sha256(payload).digest()
            git("lfs", "fsck", cwd=clone)
            git("fsck", "--strict", cwd=clone)
            passed("HTTP clone checkout and Git/LFS integrity")
            git("tag", "-a", "probe-v1", "-m", "annotated verification tag", cwd=work)
            git("notes", "add", "-m", "verification note", cwd=work)
            git("push", "origin", "refs/tags/probe-v1", "refs/notes/commits", cwd=work)
            git("fetch", "origin", "refs/notes/*:refs/notes/*", cwd=clone)
            assert git("notes", "show", cwd=clone) == "verification note"
            passed("annotated tags and Git notes")
            git("checkout", "-b", "feature/probe", cwd=work)
            (work / "feature.txt").write_text("feature verification\n")
            git("add", "feature.txt", cwd=work)
            git("commit", "-m", "test: verify branch and pull request", cwd=work)
            git("push", "origin", "feature/probe", cwd=work)
            pr = api(repo_path + "/pulls", "POST", {"title": "Git verification", "head": "feature/probe", "base": "main", "body": "Disposable protocol test."})
            for _ in range(40):
                if api(repo_path + "/pulls/" + str(pr["number"])).get("mergeable"):
                    break
                time.sleep(0.25)
            api(repo_path + "/pulls/" + str(pr["number"]) + "/merge", "POST", {"Do": "merge"})
            git("pull", "--ff-only", cwd=clone)
            assert (clone / "feature.txt").read_text() == "feature verification\n"
            git("push", "origin", "--delete", "feature/probe", cwd=work)
            git("fetch", "--prune", cwd=clone)
            assert "origin/feature/probe" not in git("branch", "-r", cwd=clone)
            passed("branch push PR merge pull and prune")
            git("clone", "--depth=1", fixture["clone_url"], str(root / "shallow"))
            assert git("rev-parse", "--is-shallow-repository", cwd=root / "shallow") == "true"
            git("clone", "--filter=blob:none", "--no-checkout", fixture["clone_url"], str(root / "partial"))
            assert git("config", "--get", "remote.origin.promisor", cwd=root / "partial") == "true"
            git("clone", "--mirror", fixture["clone_url"], str(root / "mirror.git"))
            git("fsck", "--strict", cwd=root / "mirror.git")
            passed("shallow partial and mirror clones")
            git("lfs", "lock", "payload.bin", cwd=work)
            locks = json.loads(git("lfs", "locks", "--json", cwd=work))
            if isinstance(locks, dict):
                locks = locks["locks"]
            assert any(lock["path"] == "payload.bin" for lock in locks)
            git("lfs", "unlock", "payload.bin", cwd=work)
            passed("LFS lock list and unlock")
            archive = api(repo_path + "/archive/main.zip", raw=True)
            with zipfile.ZipFile(io.BytesIO(archive)) as z:
                assert any(name.endswith("/README.md") for name in z.namelist())
            passed("repository ZIP archive")
            api("/admin/users", "POST", {"username": reader_name, "email": reader_name + "@localhost.invalid", "password": reader_password, "must_change_password": False, "send_notify": False})
            resources["reader"] = True
            team = api("/orgs/" + owner + "/teams", "POST", {"name": "read-" + suffix, "display_name": "Read-only verification", "permission": "read", "units_map": {"repo.code": "read"}})
            resources["team"] = team["id"]
            api("/teams/" + str(team["id"]) + "/members/" + reader_name, "PUT")
            api("/teams/" + str(team["id"]) + "/repos/" + owner + "/" + repo, "PUT")
            if args.demo_credentials_file:
                with args.demo_credentials_file.open("x") as handle:
                    os.chmod(args.demo_credentials_file, 0o600)
                    json.dump({"username": reader_name, "password": reader_password,
                               "team_id": team["id"], "repository": owner + "/" + repo}, handle)
            reader_env = auth_env(reader_name, reader_password)
            git("ls-remote", fixture["clone_url"], env=reader_env)
            git("push", "origin", "HEAD:refs/heads/denied-reader", cwd=work, env=reader_env, fail=True)
            anonymous = auth_env("", "")
            git("ls-remote", fixture["clone_url"], env=anonymous, fail=True)
            passed("read group clone allowed push denied and anonymous access denied")
            if args.ssh_known_hosts:
                git_bin = Path(shutil.which("git")).resolve()
                candidates = [git_bin.parent / "ssh-keygen", Path("C:/Program Files/Git/usr/bin/ssh-keygen.exe")]
                keygen = next((str(p) for p in candidates if p.exists()), shutil.which("ssh-keygen"))
                if not keygen:
                    raise RuntimeError("ssh-keygen is required for SSH verification")
                private = root / "client-key"
                subprocess.run([keygen, "-q", "-t", "ed25519", "-N", "", "-f", str(private)], check=True, capture_output=True)
                key = api("/user/keys", "POST", {"title": "gitsss-verification-" + suffix, "key": private.with_suffix(".pub").read_text()})
                key_id = key["id"]
                ssh_env = admin_env.copy()
                ssh_binary = Path(keygen).with_name("ssh.exe" if os.name == "nt" else "ssh")
                ssh_env["GIT_SSH_COMMAND"] = '"' + ssh_binary.as_posix() + '" -i "' + private.as_posix() + '" -o IdentitiesOnly=yes -o StrictHostKeyChecking=yes -o UserKnownHostsFile="' + args.ssh_known_hosts.resolve().as_posix() + '"'
                ssh_clone = root / "ssh"
                git("clone", fixture["ssh_url"], str(ssh_clone), env=ssh_env)
                assert (ssh_clone / "payload.bin").read_bytes() == payload
                git("push", "origin", "HEAD:refs/heads/ssh-probe", cwd=ssh_clone, env=ssh_env)
                git("push", "origin", "--delete", "ssh-probe", cwd=ssh_clone, env=ssh_env)
                passed("SSH verified host key clone LFS checkout push and branch deletion")
            else:
                checks.append({"check": "SSH", "status": "not-run", "reason": "supply authenticated server host keys via --ssh-known-hosts"})
            if args.actions:
                git("checkout", "main", cwd=work)
                git("pull", "--ff-only", cwd=work)
                workflow = work / ".gitea" / "workflows" / "verification.yml"
                workflow.parent.mkdir(parents=True)
                workflow.write_text("name: Verification\non: [push, workflow_dispatch]\njobs:\n  verify:\n    runs-on: " + args.runner_label + "\n    steps:\n      - uses: actions/checkout@v4\n      - run: test -s README.md && echo verified\n      - uses: actions/upload-artifact@v4\n        with:\n          name: verification-evidence\n          path: README.md\n")
                git("add", ".gitea/workflows/verification.yml", cwd=work)
                git("commit", "-m", "test: verify isolated Actions execution", cwd=work)
                head = git("rev-parse", "HEAD", cwd=work)
                git("push", "origin", "main", cwd=work)
                deadline = time.monotonic() + 180
                run = None
                while time.monotonic() < deadline:
                    runs = api(repo_path + "/actions/runs?limit=20").get("workflow_runs", [])
                    run = next((r for r in runs if r["head_sha"] == head), None)
                    if run and run.get("conclusion"):
                        break
                    time.sleep(2)
                if not run or run.get("conclusion") != "success":
                    raise RuntimeError("isolated Actions run did not succeed: " + json.dumps({k: run.get(k) for k in ["id", "status", "conclusion", "html_url"]} if run else None))
                artifacts = api(repo_path + "/actions/runs/" + str(run["id"]) + "/artifacts")
                assert artifacts["total_count"] == 1
                passed("isolated Actions checkout shell execution and artifact upload")
        result = {"status": "pass", "url": base, "repository": owner + "/" + repo, "checks": checks}
    except Exception as exc:
        message = str(exc).replace(token, "[redacted]").replace(reader_password, "[redacted]")
        result = {"status": "fail", "checks": checks, "error": message}
        exit_code = 1
    finally:
        cleanup_errors = []
        for path in (["/user/keys/" + str(key_id)] if key_id else []) + (["/teams/" + str(resources["team"])] if "team" in resources and not args.demo_credentials_file else []) + (["/admin/users/" + reader_name] if resources.get("reader") and not args.demo_credentials_file else []) + ([repo_path] if resources.get("repo") and not args.keep_fixture else []):
            try:
                api(path, "DELETE")
            except Exception as exc:
                cleanup_errors.append(str(exc).replace(token, "[redacted]").replace(reader_password, "[redacted]"))
        if cleanup_errors:
            result["status"] = "fail"
            result["cleanup_errors"] = cleanup_errors
            exit_code = 1
    print(json.dumps(result, indent=2))
    return exit_code


if __name__ == "__main__":
    raise SystemExit(main())
