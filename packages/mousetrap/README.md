# mousetrap

The MyAnonaMouse (MAM) grab authority: it decides what to pull from MAM and enforces
account safety, for both reactive and proactive demand. A clean-slate Go rewrite —
this document is the requirements it must satisfy, not a description of the previous
implementation.

## Purpose

MAM is a private tracker with account-health rules (unsatisfied-torrent limits, ratio
buffer, a finite stock of Personal Freeleech wedges). Getting the most out of it
without endangering the account requires a single place that makes every grab
decision under one coherent policy. mousetrap is that place.

## Functional requirements

- **R1 — Single grab authority.** Every MAM grab goes through mousetrap. No other
  tool touches MAM's download endpoint. This is the precondition that makes
  account-safety policy enforceable at all.

- **R2 — Reactive intake.** Accept pushed announcements the instant they appear and
  decide, per announcement, whether to grab.

- **R3 — Proactive intake.** Present as an indexer to a library manager so it can
  search MAM and request grabs on demand, under the identical policy. Interface must
  be compatible with the standard arr/library-manager ecosystem.

- **R4 — Acceptance policy.** Filter candidates by content attributes (media kind,
  category, format, language, size, freeleech/VIP status, title patterns). A rejected
  candidate records the reason. Entirely config-driven.

- **R5 — Account-safety grab policy.** Before grabbing, enforce, from live account
  state:
  - an unsatisfied-torrent ceiling that stays under MAM's hard limit with margin,
  - a ratio-buffer floor for grabs that cost ratio,
  - a wedge-spending strategy that makes selected grabs freeleech while keeping a
    reserve,
  - correct freeleech accounting — including that a VIP-freeleech release is free
    only when the account itself is VIP.
  Decisions account for **optimistic in-flight state** so a burst of grabs cannot
  overshoot a limit before MAM's own counters catch up.

- **R6 — Fetch and deliver.** On approval, fetch the torrent (spending a wedge when
  the policy chose to) and deliver it — to the download client for reactive grabs, or
  back to the requester for proactive ones.

- **R7 — Idempotent grabs.** Never grab the same torrent twice — across restarts and
  across both intake paths.

- **R8 — Account awareness.** Maintain a current view of account state (ratio,
  buffer, unsatisfied count/limit, wedge stock, class, bonus/wedge ledger) to drive
  policy and observability.

- **R9 — Observability.** Expose metrics and structured logs sufficient to tune the
  acceptance and grab policies from real traffic, and to alert on failures.

- **R10 — Declarative policy.** Every threshold, list, and toggle lives in host
  config, explicit even at its default value. No runtime-mutable config, no hidden
  defaults.

## Constraints

- The MAM session is **single-owner and serialized**: MAM rotates the session token
  on every request, so requests cannot run concurrently and the rotated token must
  always be persisted.
- Runtime state lives on **local disk only** (never network storage).
- Config is declarative and read-only at runtime; the state store holds runtime data
  only — never config, never metrics.
- Distributed as a **single static binary**.

## Invariants

- MAM is never exposed as a direct indexer to any other tool — only through
  mousetrap's gated interface.
- A held decision is **safe and retryable**: the requester may ask again later, and
  nothing that violates account-safety policy is ever grabbed.

## Non-goals

- **Library management** — organizing, hardlinking, metadata enrichment. That belongs
  to the downstream library manager; mousetrap only decides and fetches.
- **Proxying an indexer manager** (e.g. Prowlarr) for MAM — mousetrap owns the MAM
  session directly; a second session would fight token rotation.
- Storing config or metrics in the state store.

## Stack

Go, single static binary. Pure-Go SQLite for state (cgo-free). Prometheus metrics.
Built via the `homelab` overlay and referenced as `pkgs.homelab.mousetrap`.
