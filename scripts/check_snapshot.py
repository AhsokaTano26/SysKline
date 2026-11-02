#!/usr/bin/env python3
"""Verify a native executable can sample real host data without database writes."""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile

binary = str(Path(sys.argv[1]).resolve())
with tempfile.TemporaryDirectory(prefix='syskline-native-') as directory:
    env = dict(os.environ, XDG_CONFIG_HOME=directory, XDG_DATA_HOME=directory, APPDATA=directory)
    config = Path(directory) / 'config.toml'
    config.write_text('')
    output = subprocess.check_output([binary, '--config', str(config), '--once', '--interval', '250ms'], env=env, text=True, timeout=30)
    snapshot = json.loads(output)
    assert snapshot['mode'] == 'live'
    assert len(snapshot['metrics']) == 10
    assert snapshot['memory_total_bytes'] > 0
    memory = snapshot['metrics']['memory']['value']
    assert memory is not None and 0 <= memory <= 100
    cpu = snapshot['metrics']['cpu']['value']
    assert cpu is None or 0 <= cpu <= 100
    assert not list(Path(directory).rglob('*.db'))
    print(f'PASS: native {sys.platform} live CPU/memory snapshot; {len(snapshot.get("warnings", []))} optional-source warnings')
