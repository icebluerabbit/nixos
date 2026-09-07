# clients

One package per external system this homelab talks to: `qbittorrent/`,
`komga/`, `homeassistant/`, and so on.

## Rules

- **Prefer an upstream client.** Where a maintained Go client exists, depend on
  it and use its API directly — do not wrap it for the sake of symmetry. The
  ones already chosen are `autobrr/go-qbittorrent`, `golift.io/starr` (Lidarr
  and the rest of the \*arr family), `maunium.net/go/mautrix`, and
  `code.gitea.io/sdk/gitea` for Forgejo.
- **Otherwise write the client here,** over `homelab/httpx`. Komga,
  Audiobookshelf, Cleanuparr, slskd, thelounge, Kiwix, CoolerControl and Home
  Assistant have no maintained Go client, so they get one here. Retry, auth,
  backoff, JSON and logging come from `httpx`; only endpoint structs and method
  signatures are written by hand.
- **Hand upstream SDKs `httpx.Client.HTTP()`** rather than letting them build
  their own transport, so every service call shares one retry and timeout
  policy.
- **A service client belongs here from its first consumer.** Unlike invented
  abstractions, its boundary is defined by the external API, so there is no
  wrong abstraction to commit to early.
- **CLIs are not libraries.** `restic` and `beets` are driven with
  `exec.Command`; do not import their source packages.

## Why its own module

`mautrix` and the \*arr SDKs are heavy. Keeping them out of `homelab` (the
`lib/` module) means `flac-util` and `sys-mcp` do not inherit them.
