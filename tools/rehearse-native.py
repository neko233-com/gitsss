#!/usr/bin/env python3
"""Validate a SQLite native takeover using an independent, private snapshot."""

import argparse
import configparser
import datetime
import hashlib
import io
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import time


def quoted(name):
    return '"' + name.replace('"', '""') + '"'


def fingerprint(connection, table, columns):
    query = "SELECT " + ",".join(map(quoted, columns)) + " FROM " + quoted(table)
    try:
        rows = connection.execute(query + " ORDER BY rowid")
    except sqlite3.OperationalError:
        rows = connection.execute(query + " ORDER BY " + ",".join(map(quoted, columns)))
    digest, count = hashlib.sha256(), 0
    for row in rows:
        digest.update(repr(tuple(row)).encode("utf-8"))
        count += 1
    return count, digest.digest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-work-path", type=Path, required=True)
    parser.add_argument("--work-path", type=Path, required=True, help="new empty rehearsal directory")
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--port", type=int, default=5098)
    parser.add_argument("--apply", action="store_true", help="create the snapshot and migrate only that copy")
    args = parser.parse_args()
    source, work, binary = args.source_work_path.resolve(), args.work_path.resolve(), args.binary.resolve()
    if work == source or source in work.parents or work in source.parents:
        raise ValueError("source and rehearsal directories must be independent")
    if not 1024 <= args.port <= 65535:
        raise ValueError("rehearsal port must be between 1024 and 65535")
    config = configparser.ConfigParser(interpolation=None, strict=False)
    config.optionxform = str
    config.read_string("[DEFAULT]\n" + (source / "custom/conf/app.ini").read_text(encoding="utf-8"))
    if config.get("database", "DB_TYPE", fallback="").lower() not in ("sqlite3", "sqlite"):
        raise ValueError("this rehearsal tool supports SQLite sources only")
    database = Path(config.get("database", "PATH", fallback=str(source / "data/gitea.db")))
    if not database.is_absolute():
        database = source / database
    database = database.resolve()
    if work == database or work in database.parents or database in work.parents:
        raise ValueError("rehearsal must not contain the source database")
    for section in config.sections():
        if config.get(section, "STORAGE_TYPE", fallback="local").lower() != "local":
            raise ValueError("map non-local storage explicitly before native rehearsal")
    if not binary.is_file() or not database.is_file():
        raise ValueError("binary and source database must exist")
    if work.exists() and any(work.iterdir()):
        raise ValueError("rehearsal directory must be new or empty")
    if not args.apply:
        print(json.dumps({"mode": "dry-run", "source_work_path": str(source), "work_path": str(work),
                          "database": "independent SQLite backup", "copy_git_lfs": False,
                          "start_server": False}, indent=2))
        return 0

    os.umask(0o077)
    for directory in (work, work / "custom/conf", work / "data", work / "reports"):
        directory.mkdir(parents=True, exist_ok=True, mode=0o700)
    target_database = work / "data/gitea.db"
    original = sqlite3.connect(database.as_uri() + "?mode=ro", uri=True)
    snapshot = sqlite3.connect(target_database)
    original.backup(snapshot)
    original.close()
    tables = [row[0] for row in snapshot.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")]
    columns = {table: [row[1] for row in snapshot.execute("PRAGMA table_info(" + quoted(table) + ")")] for table in tables}
    before = {table: fingerprint(snapshot, table, names) for table, names in columns.items()}
    version = snapshot.execute("SELECT version FROM version WHERE id=1").fetchone()[0]
    snapshot.close()

    for key, value in list(config.defaults().items()):
        config[config.default_section][key] = value.replace(str(source), str(work))
    config[config.default_section]["WORK_PATH"] = str(work)
    for section in config.sections():
        for key, value in list(config.items(section)):
            if key in config.defaults():
                continue
            config.set(section, key, value.replace(str(source), str(work)))
    overrides = {
        "database": {"PATH": str(target_database)},
        "repository": {"ROOT": str(work / "data/gitea-repositories")},
        "server": {"APP_DATA_PATH": str(work / "data"), "HTTP_ADDR": "127.0.0.1", "HTTP_PORT": str(args.port),
                   "HTTP_ADDITIONAL_PORTS": "", "ROOT_URL": "http://127.0.0.1:" + str(args.port) + "/",
                   "LOCAL_ROOT_URL": "http://127.0.0.1:" + str(args.port) + "/", "DISABLE_SSH": "true",
                   "SSH_ROOT_PATH": str(work / "data/ssh"), "SSH_CREATE_AUTHORIZED_KEYS_FILE": "false",
                   "SSH_CREATE_AUTHORIZED_PRINCIPALS_FILE": "false"},
        "actions": {"BUILTIN_RUNNER_ENABLED": "false"},
        "session": {"COOKIE_NAME": "gitsss_rehearsal_" + str(args.port)},
        "i18n": {"COOKIE_NAME": "gitsss_rehearsal_language_" + str(args.port)},
        "security": {"COOKIE_REMEMBER_NAME": "gitsss_rehearsal_remember_" + str(args.port)},
        "cron": {"ENABLED": "false"},
        "mailer": {"ENABLED": "false"},
        "webhook": {"ALLOWED_HOST_LIST": "gitsss-rehearsal.invalid"},
        "log": {"ROOT_PATH": str(work / "logs")},
    }
    for section, values in overrides.items():
        if not config.has_section(section):
            config.add_section(section)
        for key, value in values.items():
            config.set(section, key, value)
    with (work / "custom/conf/app.ini").open("x", encoding="utf-8") as output:
        # Preserve Gitea's unnamed root section; ConfigParser calls it DEFAULT.
        text = io.StringIO()
        config.write(text)
        output.write(text.getvalue().replace("[DEFAULT]\n", "", 1))
    started = time.monotonic()
    with (work / "reports/migration.log").open("x", encoding="utf-8") as log:
        run = subprocess.run([str(binary), "migrate", "--work-path", str(work)], stdout=log, stderr=log, timeout=300)
    if run.returncode:
        raise RuntimeError("snapshot migration failed; inspect the private rehearsal migration log")
    migrated = sqlite3.connect(target_database.as_uri() + "?mode=ro", uri=True)
    unchanged = {table: before[table] == fingerprint(migrated, table, names) for table, names in columns.items()}
    result = {"status": "pass" if all(unchanged.values()) else "fail",
              "collected_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "source_db_version": version, "target_db_version": migrated.execute("SELECT version FROM version WHERE id=1").fetchone()[0],
              "tables_verified": len(columns), "record_counts": {table: before[table][0] for table in tables},
              "all_original_columns_and_row_ids_unchanged": all(unchanged.values()),
              "changed_tables": [table for table, same in unchanged.items() if not same],
              "migration_seconds": round(time.monotonic() - started, 3),
              "git_and_lfs_bytes_copied": False, "source_service_modified": False}
    migrated.close()
    report = work / "reports/native-compatibility.json"
    report.write_text(json.dumps(result, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(result, indent=2))
    return 0 if result["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
