# Cross-platform binaries and first GitHub Release

User requests macOS, Linux and Windows binaries and an actual Release. Publish
the first version as v0.1.0 (no existing tags/releases were found). Target amd64
and arm64 on each OS, preserving single executables and terminal-only operation.

- [x] Add a native source factory: retain Linux procfs; use pinned pure-Go
      gopsutil on macOS/Windows. Preserve rate baselines, counter reset/hotplug,
      NULL and filter semantics. Windows Unix load averages are n/a.
- [x] Test portable conversions/rates with deterministic snapshots; exercise
      actual macOS collection and add native Linux/macOS/Windows CI checks.
      Normalize Windows SQLite URI paths and platform data/config directories.
- [x] Extend release packaging to six targets, tar.gz on Unix and zip with .exe
      on Windows, plus raw binaries and a complete SHA256 manifest. Validate
      archive members, executable modes, file architectures and version labels.
- [x] Gate builds on native OS tests/snapshots. Build every target with CGO=0;
      publish only after all checks pass. Upload release notes explaining any
      platform-specific metric semantics and installation commands.
- [x] Run tests/race/vet/build/package/terminal checks, review, commit/push, tag
      v0.1.0 and wait for the actual GitHub Release and all assets to publish.

Release publication is explicitly authorized by this user request. Do not report
success for an artifact-only build or unexecuted Release step.

Local validation: full tests/race/vet and actionlint passed; six CGO-free
executables and all 12 checksums verified. macOS release binary sampled real
CPU/memory without warnings; macOS and Linux ARM64 Docker live PTY checks passed.
Code review caught Windows IRQ double counting; corrected with a regression test
and re-reviewed.

Cloud validation: main run 41437625073 and tagged run 41438198255 both succeeded.
The tag resolves to 9ad890bfba361f944965ec827c418872a19f6dc0. All three native OS
checks, six-target package verification and the tagged Release publication passed.
Release https://github.com/AhsokaTano26/SysKline/releases/tag/v0.1.0 is public,
non-draft and non-prerelease. All 13 expected assets are uploaded; downloaded
SHA256SUMS matches its GitHub digest and all 12 asset digests match that manifest.
