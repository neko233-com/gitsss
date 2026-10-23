#!/usr/bin/env python3
"""Create the versioned source archive from Git-tracked files only."""
import gzip
import io
import subprocess
import sys
import tarfile
from pathlib import Path


def tracked(root):
    try:
        output = subprocess.run(["git", "-C", str(root), "ls-files", "-z"], capture_output=True, check=True).stdout
    except (OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(f"source archive requires a Git working tree: {error}") from error
    return sorted(name for name in output.decode().split("\0") if name)


def main():
    folder = Path(sys.argv[1]).resolve(strict=True)
    version = sys.argv[2]
    root = Path(__file__).resolve().parents[1]
    prefix = f'gitsss-src-{version}'
    output = folder / f'{prefix}.tar.gz'
    with output.open('wb') as raw, gzip.GzipFile(fileobj=raw, mode='wb', compresslevel=6, mtime=0) as compressed:
        with tarfile.open(fileobj=compressed, mode='w', format=tarfile.PAX_FORMAT) as archive:
            for relative in tracked(root):
                source = root / relative
                if not source.is_file():
                    continue
                info = tarfile.TarInfo(f'{prefix}/{relative}')
                info.size = source.stat().st_size
                info.mode = 0o755 if relative.endswith(('.sh', '.py')) else 0o644
                info.mtime = 0
                info.uid = info.gid = 0
                info.uname = info.gname = ''
                with source.open('rb') as stream:
                    archive.addfile(info, stream)
            data = f'{version}\n'.encode()
            info = tarfile.TarInfo(f'{prefix}/VERSION')
            info.size, info.mtime = len(data), 0
            info.uid = info.gid = 0
            info.uname = info.gname = ''
            archive.addfile(info, io.BytesIO(data))
    print(f'packaged {output.name}')


if __name__ == '__main__':
    main()
