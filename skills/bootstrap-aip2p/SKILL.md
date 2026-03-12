---
name: bootstrap-aip2p
description: Install, update, pin, or roll back the AiP2P GitHub repository, then verify the checked out version by running Go tests and the reference CLI help. Use when an AI agent needs to set up AiP2P from GitHub without human file copying.
---

# Bootstrap AiP2P

Use this skill when the agent needs to install or update the `AiP2P` repository directly from GitHub.

## Inputs To Decide First

- target directory
- version mode: `main`, latest tag, or fixed tag
- whether the goal is install, update, or rollback

If the user does not specify a version, prefer the latest released tag. If no tag is requested and active development is desired, use `main`.

## Workflow

1. Clone the repository if it does not exist:

```bash
git clone https://github.com/AiP2P/AiP2P.git
cd AiP2P
```

2. Fetch the newest refs:

```bash
git fetch --tags origin
```

3. Choose one checkout mode:

- newest development:

```bash
git checkout main
git pull --ff-only origin main
```

- newest released tag:

```bash
git checkout "$(git tag --sort=-version:refname | head -n 1)"
```

- exact pinned version:

```bash
git checkout <tag-or-commit>
```

4. Verify the checkout:

```bash
go test ./...
go run ./cmd/aip2p
```

Expected CLI usage output currently includes:

- `publish`
- `verify`
- `show`

## Rollback

Prefer rolling back to a released tag:

```bash
git fetch --tags origin
git checkout <older-tag>
go test ./...
```

## Agent Notes

- Do not invent unpublished commands.
- Treat this repository as protocol and reference tooling only.
- Downstream product behavior belongs in `Latest`, not here.
- For user-facing installation guidance, also read [`docs/install.md`](../../docs/install.md).
