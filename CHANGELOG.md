# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- automated patch release on every PR merge to main; `release:minor` / `release:major` labels raise the bump (refs #91)
- staticcheck and actionlint as required CI gates (refs #97)
- dependabot docker ecosystem coverage for `.devcontainer/Dockerfile` (refs #97)
- darwin success-path bats test for install.sh bootstrap sequence (refs #97)
- CI badge in README (refs #97)

### Fixed

- Golden flow test flake: replaced `time.Sleep` with `teatest.WaitFor` (refs #97)
- SA5011: `t.Error`-then-deref pattern replaced with `t.Fatal` in forms test (refs #97)
- S1011: accumulation loop replaced with `append(lines, pairs...)` in extra test (refs #97)
