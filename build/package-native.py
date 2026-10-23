#!/usr/bin/env python3
"""Bundle matching native server and hook binaries for platform-based updates."""
from concurrent.futures import ThreadPoolExecutor
import gzip
import hashlib
import io
from pathlib import Path
import sys
import tarfile


def main():
    folder = Path(sys.argv[1]).resolve(strict=True)
    version = sys.argv[2]
    root = Path(__file__).resolve().parents[1]
    platforms = [('linux', 'amd64'), ('linux', 'arm64'), ('darwin', 'amd64'),
                 ('darwin', 'arm64'), ('windows', 'amd64'), ('windows', 'arm64'), ('windows', '386')]

    def bundle(platform):
        system, architecture = platform
        extension = '.exe' if system == 'windows' else ''
        binary = folder / f'gitsss-{version}-{system}-{architecture}{extension}'
        hook = folder / f'gitsss-hook-{version}-{system}-{architecture}{extension}'
        output = folder / f'gitsss-{version}-{system}-{architecture}.tar.gz'
        instructions = ('Keep gitsss and gitsss-hook in the same directory, using these fixed filenames.\n'
                        'Preserve existing configuration and repository/LFS paths when upgrading.\n'
                        'See docs/features/push-throughput.md for the Linux CGO WAL/NORMAL profile.\n').encode()
        with output.open('wb') as target, gzip.GzipFile(fileobj=target, mode='wb', compresslevel=3, mtime=0) as compressed:
            with tarfile.open(fileobj=compressed, mode='w', format=tarfile.USTAR_FORMAT) as archive:
                for source, name in [(binary, 'gitsss'+extension), (hook, 'gitsss-hook'+extension), (root/'LICENSE', 'LICENSE')]:
                    info = tarfile.TarInfo(name)
                    info.size = source.stat().st_size
                    info.mode = 0o755 if name.endswith(extension) and name.startswith('gitsss') else 0o644
                    with source.open('rb') as stream:
                        archive.addfile(info, stream)
                info = tarfile.TarInfo('INSTALL.md')
                info.size, info.mode = len(instructions), 0o644
                archive.addfile(info, io.BytesIO(instructions))
        with output.open('rb') as stream:
            digest = hashlib.file_digest(stream, 'sha256').hexdigest()
        Path(str(output)+'.sha256').write_text(digest+'  '+output.name+'\n', encoding='utf-8', newline='\n')
        return output.name

    with ThreadPoolExecutor(max_workers=4) as pool:
        for name in pool.map(bundle, platforms):
            print('packaged '+name)


if __name__ == '__main__':
    main()
