#!/usr/bin/env python3
"""Verify a native release matrix and write a readback-checked SHA256SUMS."""
import argparse
import hashlib
import lzma
import subprocess
import tarfile
from pathlib import Path

PLATFORMS = [
    ('linux', 'amd64', ''),
    ('linux', 'arm64', ''),
    ('darwin', 'amd64', ''),
    ('darwin', 'arm64', ''),
    ('windows', 'amd64', '.exe'),
    ('windows', 'arm64', '.exe'),
    ('windows', '386', '.exe'),
]
GOGIT_PLATFORMS = [
    ('windows', 'amd64', '.exe'),
    ('windows', 'arm64', '.exe'),
    ('windows', '386', '.exe'),
]


def sha256(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def check_file(folder, name):
    path = folder / name
    if not path.is_file():
        raise SystemExit(f'missing release artifact: {name}')
    if path.stat().st_size == 0:
        raise SystemExit(f'empty release artifact: {name}')


def check_checksum(folder, name):
    path = folder / name
    checksum = path.with_name(name + '.sha256')
    expected = sha256(path)
    if not checksum.is_file():
        checksum.write_text(f'{expected}  {name}\n', encoding='utf-8')
    fields = checksum.read_text(encoding='utf-8').split()
    if len(fields) != 2 or fields[0] != expected or fields[1].lstrip('*') != name:
        raise SystemExit(f'checksum mismatch or malformed: {checksum.name}')


def check_xz(folder, name):
    archive = folder / (name + '.xz')
    digest = hashlib.sha256()
    with lzma.open(archive, 'rb') as stream:
        while block := stream.read(1024 * 1024):
            digest.update(block)
    if digest.hexdigest() != sha256(folder / name):
        raise SystemExit(f'xz payload does not match {name}')


def check_bundle(folder, name, binary, hook, executable):
    bundle = folder / name
    expected = {'gitsss' + executable, 'gitsss-hook' + executable, 'LICENSE', 'INSTALL.md'}
    if not bundle.is_file():
        raise SystemExit(f'missing bundle: {name}')
    with tarfile.open(bundle, 'r:gz') as archive:
        members = {member.name: member for member in archive.getmembers()}
        if set(members) != expected:
            raise SystemExit(f'{name} members are {sorted(members)}, expected {sorted(expected)}')
        for member in members.values():
            if not member.isfile() or '/' in member.name or member.name.startswith('.'):
                raise SystemExit(f'{name} contains an unsafe member: {member.name}')
            if member.name.startswith('gitsss') and member.mode & 0o111 == 0:
                raise SystemExit(f'{name} member is not executable: {member.name}')
        for member_name, source in (('gitsss' + executable, folder / binary),
                                    ('gitsss-hook' + executable, folder / hook)):
            digest = hashlib.sha256()
            stream = archive.extractfile(members[member_name])
            while block := stream.read(1024 * 1024):
                digest.update(block)
            if digest.hexdigest() != sha256(source):
                raise SystemExit(f'{name} member {member_name} does not match {source.name}')
    check_checksum(folder, name)


def check_buildinfo(folder, name, version, server):
    try:
        output = subprocess.run(['go', 'version', '-m', str(folder / name)], check=True,
                                capture_output=True, text=True).stdout
    except (OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(f'cannot read build info for {name}: {error}') from error
    if '\tpath\tgithub.com/neko233-com/gitsss' not in output and '\tmod\tgithub.com/neko233-com/gitsss' not in output:
        raise SystemExit(f'{name} does not belong to the gitsss module')
    if server and f'-X main.Version={version}' not in output:
        raise SystemExit(f'{name} does not embed version {version}')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('folder')
    parser.add_argument('version')
    parser.add_argument('--gogit', action='store_true')
    parser.add_argument('--skip-buildinfo', action='store_true')
    args = parser.parse_args()
    folder = Path(args.folder).resolve(strict=True)
    version = args.version

    expected = set()
    bundles = []
    for system, architecture, extension in PLATFORMS:
        binary = f'gitsss-{version}-{system}-{architecture}{extension}'
        hook = f'gitsss-hook-{version}-{system}-{architecture}{extension}'
        bundle = f'gitsss-{version}-{system}-{architecture}.tar.gz'
        for name in (binary, hook):
            expected.update({name, name + '.sha256', name + '.xz', name + '.xz.sha256'})
        expected.update({bundle, bundle + '.sha256'})
        bundles.append((binary, hook, bundle, extension))
    if args.gogit:
        for system, architecture, extension in GOGIT_PLATFORMS:
            name = f'gitsss-{version}-gogit-{system}-{architecture}{extension}'
            expected.update({name, name + '.sha256', name + '.xz', name + '.xz.sha256'})
    source = f'gitsss-src-{version}.tar.gz'
    expected.update({source, source + '.sha256'})

    for name in sorted(expected):
        if not name.endswith('.sha256'):
            check_file(folder, name)
    for path in sorted(folder.iterdir()):
        if path.is_file() and path.name.startswith('gitsss-') and path.name not in expected:
            raise SystemExit(f'stale or unexpected artifact from another version: {path.name}')

    for binary, hook, bundle, extension in bundles:
        for name in (binary, hook):
            check_checksum(folder, name)
            check_xz(folder, name)
            if not args.skip_buildinfo:
                check_buildinfo(folder, name, version, server=name == binary)
        check_bundle(folder, bundle, binary, hook, extension)
    if args.gogit:
        for system, architecture, extension in GOGIT_PLATFORMS:
            name = f'gitsss-{version}-gogit-{system}-{architecture}{extension}'
            check_checksum(folder, name)
            check_xz(folder, name)
            if not args.skip_buildinfo:
                check_buildinfo(folder, name, version, server=True)
    check_checksum(folder, source)

    names = [source]
    for binary, hook, bundle, _ in bundles:
        names.extend((binary, binary + '.xz', hook, hook + '.xz', bundle))
    if args.gogit:
        for system, architecture, extension in GOGIT_PLATFORMS:
            name = f'gitsss-{version}-gogit-{system}-{architecture}{extension}'
            names.extend((name, name + '.xz'))
    sums = folder / 'SHA256SUMS'
    lines = [f'{sha256(folder / name)}  {name}' for name in sorted(names)]
    sums.write_text('\n'.join(lines) + '\n', encoding='utf-8')
    for line in sums.read_text(encoding='utf-8').splitlines():
        digest, name = line.split()
        if sha256(folder / name) != digest:
            raise SystemExit(f'SHA256SUMS readback failed for {name}')
    print(f'verified {len(bundles)} platform bundles, {len(expected)} artifacts, and SHA256SUMS')


if __name__ == '__main__':
    main()
