#!/usr/bin/env python3
"""Package six prebuilt native executables with docs and SHA256 checksums."""
import hashlib
from pathlib import Path
import re
import sys
import tarfile
import zipfile

TARGETS = [(system, arch) for system in ('darwin', 'linux', 'windows') for arch in ('amd64', 'arm64')]
ROOT = Path(__file__).resolve().parent.parent


def digest(path):
    sha = hashlib.sha256()
    with path.open('rb') as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b''):
            sha.update(block)
    return sha.hexdigest()


def package(version):
    if not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._+-]{0,63}', version):
        raise ValueError('invalid version label')
    dist = ROOT / 'dist'
    assets = []
    for system, arch in TARGETS:
        executable = 'syskline.exe' if system == 'windows' else 'syskline'
        binary = dist / f'syskline-{system}-{arch}{".exe" if system == "windows" else ""}'
        files = [(binary, executable), (ROOT / 'LICENSE', 'LICENSE'),
                 (ROOT / 'README.md', 'README.md'),
                 (ROOT / 'configs/config.example.toml', 'configs/config.example.toml')]
        extension = 'zip' if system == 'windows' else 'tar.gz'
        archive = dist / f'syskline_{version}_{system}_{arch}.{extension}'
        if system == 'windows':
            with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as output:
                for source, name in files:
                    output.write(source, name)
        else:
            with tarfile.open(archive, 'w:gz') as output:
                for source, name in files:
                    info = output.gettarinfo(str(source), arcname=name)
                    info.mode = 0o755 if name == executable else 0o644
                    info.uid = info.gid = 0
                    info.uname = info.gname = ''
                    with source.open('rb') as stream:
                        output.addfile(info, stream)
        assets.extend([binary, archive])
    manifest = dist / 'SHA256SUMS'
    manifest.write_text(''.join(f'{digest(path)}  {path.name}\n' for path in sorted(assets)))
    print(f'Packaged 6 executables and 6 archives for {version}; SHA256SUMS written.')


if __name__ == '__main__':
    package(sys.argv[1])
