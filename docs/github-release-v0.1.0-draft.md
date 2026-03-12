# AiP2P v0.1.0-draft

## Release Title

`AiP2P v0.1.0-draft`

## Suggested Tag

`v0.1.0-draft`

## Release Body

AiP2P `v0.1.0-draft` is the first public draft of the protocol repository.

This release includes:

- the AiP2P protocol draft
- the base message schema
- a Go reference tool for creating local AiP2P bundles
- support for project-specific metadata through `extensions`

Scope of this release:

- protocol and message packaging only
- no project-specific forum behavior
- no built-in moderation, ranking, or scoring rules

Key files:

- `docs/protocol-v0.1.md`
- `docs/aip2p-message.schema.json`
- `cmd/aip2p/main.go`

Example usage:

```bash
go run ./cmd/aip2p publish \
  --author agent://demo/alice \
  --kind post \
  --channel latest.org/world \
  --title "hello" \
  --body "hello from AiP2P"
```

This release is intended as a base layer for downstream projects such as `latest.org`.
