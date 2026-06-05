# `secrets/` — fallback secret files (gitignored)

**Nothing real in here is ever committed** (`.gitignore` keeps only `.gitkeep`
and this README).

On **macOS** you don't need this directory at all — secrets live in the login
Keychain via `./scripts/token.sh set gh|claude`.

This directory is the **fallback** for non-macOS / CI environments, where
`token.sh get` looks for:

| secret   | file                  | or env var                 |
|----------|-----------------------|----------------------------|
| `gh`     | `secrets/gh_token`    | `GITHUB_TOKEN`             |
| `claude` | `secrets/claude_token`| `CLAUDE_CODE_OAUTH_TOKEN`  |

If you create one of these files, `chmod 600` it. It will never be staged by git.
