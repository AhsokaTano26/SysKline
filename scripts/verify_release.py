#!/usr/bin/env python3
"""Inspect real release archives, architectures, permissions and all checksums."""
import hashlib
from pathlib import Path
import struct
import sys
import tarfile
import zipfile


def verify(root, version):
    expected = set()
    for system in ('darwin', 'linux', 'windows'):
        for arch in ('amd64', 'arm64'):
            exe = 'syskline.exe' if system == 'windows' else 'syskline'
            binary_name = f'syskline-{system}-{arch}{".exe" if system == "windows" else ""}'
            extension = 'zip' if system == 'windows' else 'tar.gz'
            archive_name = f'syskline_{version}_{system}_{arch}.{extension}'
            expected.update([binary_name, archive_name])
            data = (root / binary_name).read_bytes()
            if system == 'linux':
                assert data[:4] == b'\x7fELF'
                assert struct.unpack_from('<H', data, 18)[0] == {'amd64': 62, 'arm64': 183}[arch]
            elif system == 'darwin':
                assert data[:4] == b'\xcf\xfa\xed\xfe'
                assert struct.unpack_from('<I', data, 4)[0] == {'amd64': 0x1000007, 'arm64': 0x100000C}[arch]
            else:
                assert data[:2] == b'MZ'
                pe = struct.unpack_from('<I', data, 60)[0]
                assert data[pe:pe+4] == b'PE\0\0'
                assert struct.unpack_from('<H', data, pe+4)[0] == {'amd64': 0x8664, 'arm64': 0xAA64}[arch]
            members = {exe, 'README.md', 'LICENSE', 'configs/config.example.toml'}
            if system == 'windows':
                with zipfile.ZipFile(root / archive_name) as archive:
                    assert set(archive.namelist()) == members
                    assert archive.read(exe) == data
            else:
                with tarfile.open(root / archive_name) as archive:
                    assert set(archive.getnames()) == members
                    assert archive.getmember(exe).mode & 0o777 == 0o755
                    assert archive.extractfile(exe).read() == data
    entries = (root / 'SHA256SUMS').read_text().splitlines()
    assert len(entries) == 12
    actual = set()
    for entry in entries:
        sha, name = entry.split()
        assert name in expected
        assert hashlib.sha256((root / name).read_bytes()).hexdigest() == sha, name
        actual.add(name)
    assert actual == expected
    print(f'PASS: six native executable formats/architectures, archives and all 12 SHA256 checksums ({version})')


if __name__ == '__main__':
    verify(Path(sys.argv[1]), sys.argv[2])
