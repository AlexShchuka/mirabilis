# mirabilis

> An autonomous Claude Code workspace on macOS: one command gives you an isolated
> dev container with the
> [`neuro-matrix`](https://github.com/AlexShchuka/neuro-matrix) harness, persistent
> memory, and a configurable egress allowlist.
> *mirabilis* honors Einstein's *annus mirabilis* — his 1905 "miracle year",
> when four landmark papers (special relativity, the photoelectric effect, Brownian motion, and
> mass–energy equivalence, E=mc²) reshaped physics in a single year.

## Requirements

macOS (Apple Silicon or Intel) with [Homebrew](https://brew.sh). Docker Desktop and
the devcontainer CLI are installed for you on first run.

## Quick start

```sh
git clone https://github.com/AlexShchuka/mirabilis.git && mirabilis/mirabilis
```

That one line is the whole setup. The first run installs prerequisites, puts
`mirabilis` on your PATH, builds the container, and signs you in to GitHub and
Claude (native flows, saved in the sandbox). After that, run it from anywhere:

```sh
mirabilis
```

## More

- `mirabilis help` — every command. Docker Desktop must be running;
- [`AGENTS.md`](AGENTS.md) — architecture and contributor rules.
- [`SECURITY.md`](SECURITY.md) — threat model and secret handling.

## License

[MIT](LICENSE)
