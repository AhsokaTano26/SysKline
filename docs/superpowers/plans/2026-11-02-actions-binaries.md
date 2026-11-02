# GitHub Actions binary builds

User request: build downloadable binary versions using GitHub Actions. Treat
"sction" as Actions in the existing GitHub repository.

- [ ] Keep tests, race/vet and terminal smoke checks as a gate; add manual workflow
      dispatch, automatic main builds and v* tag builds.
- [ ] Build static Linux amd64/arm64 binaries; archive each with a runnable
      `syskline`, README, LICENSE and example configuration; checksum all assets.
- [ ] Upload downloadable Actions artifacts for regular builds. Only a pushed
      v* tag may publish a GitHub Release, with contents:write limited to that job.
- [ ] Validate workflow syntax, run local tests/build/archive checks, inspect the
      project files, commit and push current implementation plus workflow so the
      requested remote build can execute. Do not create an arbitrary release tag.
- [ ] Follow the first remote run to completion, inspect its artifacts and verify
      downloaded assets. Fix any actual failures before reporting success.

Use only repository-scoped GitHub credentials; never print tokens. Workflow
version input is passed through environment variables and validated before use.
Version defaults to dev-<short SHA> for ordinary builds and the tag for releases.
