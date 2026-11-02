#!/usr/bin/env python3
"""Exercise the real TUI in a Unix PTY; no browser or third-party packages."""
import argparse
import fcntl
import os
from pathlib import Path
import pty
import select
import signal
import sqlite3
import struct
import tempfile
import termios
import time


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", nargs="?", default="bin/syskline")
    parser.add_argument("--live", action="store_true", help="use real Linux metrics")
    args = parser.parse_args()
    binary = str(Path(args.binary).resolve())
    with tempfile.TemporaryDirectory(prefix="syskline-smoke-") as temp:
        database = str(Path(temp) / "history.db")
        command = [binary, "--ascii", "--no-color", "--interval", "250ms"]
        command += ["--db", database] if args.live else ["--demo"]
        pid, fd = pty.fork()
        if pid == 0:
            env = dict(os.environ, TERM="xterm-256color", XDG_CONFIG_HOME=temp)
            os.execve(binary, command, env)
        transcript = bytearray()
        reaped = False

        def resize(width, height):
            fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", height, width, 0, 0))
            os.kill(pid, signal.SIGWINCH)

        def expect(text, timeout=10):
            pending = bytearray()
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                if select.select([fd], [], [], 0.1)[0]:
                    try:
                        chunk = os.read(fd, 65536)
                    except OSError:
                        break
                    if not chunk:
                        break
                    pending.extend(chunk)
                    transcript.extend(chunk)
                    if text in pending:
                        return
            raise AssertionError(f"TUI did not render {text!r}; tail={pending[-1000:]!r}")

        def send(keys, expected):
            os.write(fd, keys)
            expect(expected)

        try:
            resize(120, 34)
            expect(b" O ")
            send(b"1", b"[1s]")
            send(b"m", b"left")
            if not args.live:
                send(b"m", b"MA5")
            send(b"l", b"LINE")
            send(b"\t", b"[MEM]")
            send(b"5", b"[5m]")
            send(b"\x1b[D", b"HISTORY")
            send(b" ", b"PAUSED")
            # Allow multiple real sample ticks while the viewport is frozen.
            time.sleep(1.2)
            send(b"?", b"TERMINAL MARKET")
            send(b"?", b"PAUSED")
            send(b"g", b"LIVE")
            resize(30, 8)
            expect(b"use at least")
            resize(100, 30)
            expect(b"SYSKLINE")
            os.write(fd, b"q")
            deadline = time.monotonic() + 5
            while time.monotonic() < deadline:
                # The terminal must keep draining while the renderer restores
                # the screen; otherwise a full PTY buffer blocks child exit.
                if select.select([fd], [], [], 0.05)[0]:
                    try:
                        transcript.extend(os.read(fd, 65536))
                    except OSError:
                        pass
                done, status = os.waitpid(pid, os.WNOHANG)
                if done:
                    reaped = True
                    assert os.waitstatus_to_exitcode(status) == 0, status
                    break
            assert reaped, "TUI did not exit after q"
            assert b"\x1b[?1049l" in transcript, "alternate screen was not restored"
            if args.live:
                with sqlite3.connect(database) as db:
                    rows = db.execute("SELECT COUNT(*) FROM samples").fetchone()[0]
                    bars = db.execute("SELECT COUNT(*) FROM candles").fetchone()[0]
                    assert rows >= 3 and bars > 0, (rows, bars)
                    assert db.execute("PRAGMA integrity_check").fetchone()[0] == "ok"
                print(f"PASS: live TUI, SQLite {rows} samples / {bars} bars, integrity ok")
            else:
                assert b"DEMO" in transcript and b"SYNTHETIC" in transcript
                print("PASS: demo TUI, seconds/MA/metric/period/line/history/pause/help/resize/quit")
        finally:
            if not reaped:
                os.kill(pid, signal.SIGKILL)
                os.waitpid(pid, 0)
            os.close(fd)


if __name__ == "__main__":
    main()
