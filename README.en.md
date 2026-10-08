# Hilvan

**Hilvan** stitches AI coding agents and accounts together. Run tasks with Claude Code, Codex and more — across several accounts and environments (Linux, Windows, WSL) — while every account stays an isolated identity, context moves between agents only as **reviewable packages you approve**, and nothing switches identity behind your back.

> *Hilvanar* (Spanish): to tack pieces together with loose stitches so the work can continue.

**Status:** pre-release — MVP milestones M0–M5 done on Linux (20/22 acceptance criteria verified). Apache-2.0. Spanish documentation lives in [`README.md`](README.md) and [`docs/`](docs/).

## Why

- Switching tools when you hit a usage limit or need a specific connector means re-explaining everything.
- Personal and work accounts must not leak into each other.
- You want to know who did what, at what cost, and review it before it lands.

Hilvan is **not** a way to pool or bypass provider quotas. It launches the **official, unmodified CLIs** with a per-profile config directory and never reads or stores your credentials.

## Features (today)

- **Profiles:** one per account and environment (`CLAUDE_CONFIG_DIR` / `CODEX_HOME`), official login flows, identity checks (`hilvan profile check`).
- **Tasks:** structured briefs, one git worktree and branch per writing task, live streaming, cost and quota-window visibility, `blocked` and `uncertain` states instead of silent retries, and cancellation.
- **Criterio:** a local decision engine (no network, about 30 µs per decision) that suggests task type, complexity, stricter data classification and diff risk, explains why, and learns from your corrections.
- **Security:** terminal-escape sanitising, secret redaction, symlink-safe atomic writes, a hash-chained audit log (`hilvan audit verify`) and explicit trust for repository hooks. See [SECURITY.md](SECURITY.md).

## Quick start

```bash
# Requires Go (the go.mod version) and git; builds a single binary with no runtime.
go build -trimpath -ldflags "-s -w" -o ~/.local/bin/hilvan ./cmd/hilvan
ln -sf ~/.local/bin/hilvan ~/.local/bin/hv        # short alias (Windows: copy hilvan.exe to hv.exe)
hilvan doctor

hilvan profile add --id claude-personal --app claude-code --accepts public,personal --use-default-login
hilvan profile add --id codex-personal  --app codex       --accepts public,personal --use-default-login
hilvan profile check

cd your-repo && hilvan init
hilvan task new "Fix login bug" --objective "…" --criteria "tests pass"
hilvan task run <id> --profile claude-personal
```

## Roadmap

Done: eligibility and layered policy (M3), approved hand-offs between profiles (M4), diff review, accept/discard and cross-review (M5). Next: Windows/WSL validation, a live TUI/web dashboard, SSH destinations and parallel runs. See [docs/plans/001-mvp.md](docs/plans/001-mvp.md).

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md) (Spanish) — Conventional Commits, DCO sign-off (`git commit -s`), `go run ./scripts/check` must pass.
