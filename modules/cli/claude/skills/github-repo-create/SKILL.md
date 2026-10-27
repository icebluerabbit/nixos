---
name: github-repo-create
description: Create and wire up a new GitHub repo under the icebluerabbit account to this homelab's standard — repo creation, Cachix cache, secrets and variables, branch protection, workflow set, and the flake contracts every repo must implement. Use when adding a new authored-binary repo, a `-flake` wrapper packaging upstream software, a Nix library, or any repo that must be consumed by the nixos config. Also covers the traps that cost real time: Actions not running until enabled in the UI, cachix name length limits, fine-grained PAT scopes, and `GITHUB_TOKEN` silently suppressing workflow triggers.
---

# Creating a new GitHub repo

Account is `icebluerabbit`. The traps below are real, not theoretical — each one cost time.

## 1. Pick the archetype

Archetype is mechanical, from the name and what the repo ships.

| Archetype       | Marker             | Ships                                               | Example            |
| :-------------- | :----------------- | :-------------------------------------------------- | :----------------- |
| `flake-wrapper` | name ends `-flake` | Packages upstream software; version tracks upstream | `cleanuparr-flake` |
| `authored-bin`  | no suffix          | A binary written here; ships release assets         | `configuratarr`    |
| `flake-lib`     | no suffix          | A flake-parts module, no derivation                 | `denful-networks`  |
| `nix-config`    | —                  | A system config, consumed by nothing                | `nixos`            |

**A `-flake` suffix means the artifact is fetched, not authored.** Check for `fetchFromGitHub` /
`fetchFromGitLab` / `fetchPypi` before deciding — that is the ground truth.

## 2. Create the repo

```bash
export GH_TOKEN=$(cat ~/.config/sops-nix/secrets/github-access-token-nixos)
gh repo create icebluerabbit/<name> --public --disable-wiki
```

`github-access-token-nixos` is the classic PAT with `repo`, `workflow`, `delete_repo`. Fine-grained
PATs cannot reliably create repos under a personal account — use the classic one here.

**⚠ Then open the repo's Actions tab in a browser and enable Actions.** The REST API reports
`actions/permissions → {"enabled": true}` on a brand-new repo _even when the UI still shows
"GitHub Actions is not currently enabled on this repository"_. Until you click enable, `push`
events silently never trigger a workflow, while `workflow_dispatch` works normally — which makes
the failure look like a workflow-trigger bug rather than a disabled feature. **Do not trust the API
for this.**

## 3. Cachix cache

```bash
# create the cache at cachix.org (web UI), then:
curl -s https://app.cachix.org/api/v1/cache/<cache-name> | jq -r '.publicSigningKeys[]'
```

Cache names are **global across all of cachix.org**, not namespaced per account, and there is a
length limit — `icebluerabbit-lazylibrarian-flake` (33 chars) was rejected, so that cache is
`icebluerabbit-lazylibrarian`. Because names can diverge from repo names, **never derive the cache
name from `github.event.repository.name`**. Store it per repo:

```bash
gh variable set CACHIX_CACHE -R icebluerabbit/<name> --body '<cache-name-minus-prefix>'
```

Workflows then reference `icebluerabbit-${{ vars.CACHIX_CACHE }}`, so `ci.yml` stays byte-identical
across every repo.

Auth tokens are **per cache**, stored in `secrets/user-secrets.yaml` as `cachix-<short-name>`, so a
leak can only write one cache.

## 4. Secrets and variables

```bash
tok=$(sops -d --extract '["cachix-<short>"]'                secrets/user-secrets.yaml)
aut=$(sops -d --extract '["github-access-token-workflows"]' secrets/user-secrets.yaml)
gh secret   set CACHIX_AUTH_TOKEN -R icebluerabbit/<name> --body "$tok"
gh secret   set AUTOMATION_TOKEN  -R icebluerabbit/<name> --body "$aut"
gh variable set CACHIX_CACHE      -R icebluerabbit/<name> --body '<cache-short>'
```

Add `RELEASE_PLEASE_TOKEN` only on repos that publish release assets.

Secrets stored in sops but declared by **no NixOS/home-manager module are never decrypted** to
`~/.config/sops-nix/secrets/`. Read them with `sops -d --extract` instead of expecting a file.

`AUTOMATION_TOKEN` is a fine-grained PAT scoped to the repos, with **Contents: write, Pull requests:
write, Workflows: write**. The workflows scope is easy to miss and is required — dependabot's
`github-actions` ecosystem opens PRs touching `.github/workflows/*`, and merging those fails
without it while every other PR succeeds.

## 5. Repo settings

```bash
gh api -X PATCH repos/icebluerabbit/<name> \
  -F delete_branch_on_merge=true -F allow_auto_merge=true -F allow_rebase_merge=false \
  -F default_branch=main
gh api -X PUT repos/icebluerabbit/<name>/actions/permissions/workflow \
  -F default_workflow_permissions=read -F can_approve_pull_request_reviews=true
```

`can_approve_pull_request_reviews` **defaults to false on a new account** and `promote.yml` fails
without it — `GraphQL: GitHub Actions is not permitted to create or approve pull requests`.

Branch protection on **both** branches, required check `Lint & Verify`:

| Branch | `strict`  | Why                                                                                 |
| :----- | :-------- | :---------------------------------------------------------------------------------- |
| `dev`  | **false** | Multiple weekly automations would otherwise perpetually invalidate each other's PRs |
| `main` | **true**  | Only moves by promote-merge, so `dev` always contains it                            |

`strict: true` on `main` **stalls any repo running release-please**, because release-please pushes
version bumps directly to `main`, leaving `dev` behind and blocking every later promote PR pending
a back-merge nobody performs. Use `strict: false` there.

Protecting `dev` is not cosmetic: `gh pr merge --auto` waits only for _required_ checks, so on an
unprotected branch auto-merge lands the PR immediately — including red ones.

## 6. The three flake contracts

Every repo implements all three, so no repo-specific step ever appears in YAML.

| Contract           | Meaning                             | No-op when                |
| :----------------- | :---------------------------------- | :------------------------ |
| `.#update-package` | Track upstream version              | nothing upstream to track |
| `.#generate-docs`  | Regenerate `docs/`                  | no docs                   |
| `flake.checks`     | **Cover everything the repo ships** | —                         |

The third is the one that bites. **`nix flake check` builds `checks.*` but only _evaluates_
`packages.*`** — a package whose `src` 404s passes. That is exactly how `unifly-flake` shipped an
unbuildable package for two months behind a green `Lint & Verify`. So:

- **artifact repos** — `checks` must transitively _build_ every artifact shipped
- **library repos** — no derivation exists, so `checks` must _exercise the module_ (see
  `denful-networks`' `test/` sub-flake)

Do not build the shipped artifact by a different route than the tested one. `configuratarr` built
its binary with `rustPlatform.buildRustPackage` while every check used crane + fenix, so the shipped
binary was compiled by a different toolchain, from a separately resolved dependency graph, and ran
zero tests.

**Rust repos use crane**, not `buildRustPackage`. `buildRustPackage` is the nixpkgs convention for
packaging _third-party_ Rust; crane is the convention for a project shipping its own flake, and it
gives package and checks one dependency build.

## 7. Workflow set

| Workflow                                       | Trigger                 | Purpose                                                                                |
| :--------------------------------------------- | :---------------------- | :------------------------------------------------------------------------------------- |
| `ci.yml`                                       | push/PR on `main`,`dev` | `nix flake check -L`; `nix fmt` diff-clean; docs diff-clean                            |
| `flake-update.yml`                             | cron Sun 03:00 UTC      | `nix flake update` → `.#update-package` → `.#generate-docs` → PR to `dev` → auto-merge |
| `promote.yml`                                  | push to `dev`           | Maintain the standing `dev` → `main` PR. **Merged by hand**                            |
| `dependabot.yml` + `dependabot-auto-merge.yml` | —                       | Actions deps, plus language ecosystem                                                  |
| `release-please.yml`                           | push to `main`          | Authored-binary repos only                                                             |

Action pins, one set everywhere:

```
actions/checkout@v7
DeterminateSystems/nix-installer-action@v22
cachix/cachix-action@v17
peter-evans/create-pull-request@v8
googleapis/release-please-action@v5
dependabot/fetch-metadata@v3
```

### `GITHUB_TOKEN` never drives anything another workflow must observe

Events raised by `GITHUB_TOKEN` **do not trigger workflows**. This has four distinct faces, all
observed in production:

1. `create-pull-request` with `GITHUB_TOKEN` → update PRs never run CI
2. `gh pr merge --auto` with `GITHUB_TOKEN` → the merge never fires `promote.yml`
3. `promote.yml`'s own `gh pr create` → the release PR is born with no checks, and where `main`
   requires one it is unmergeable until an unrelated push re-fires it
4. a plain `git push` from CI under the checkout-persisted token → **neither** `ci.yml` nor
   `promote.yml` runs

Consequence when it goes unnoticed: `denful-networks` accumulated seven weekly lockfile bumps with
CI dead for two months, and one of them broke the test suite silently.

Use `AUTOMATION_TOKEN` for all four. `dependabot/fetch-metadata` may keep `GITHUB_TOKEN` — nothing
observes it.

### Gate dependabot auto-merge on update type

`fetch-metadata` exists to classify the update; if its output is never read, **every** dependabot PR
merges including majors:

```yaml
- name: Enable Auto-Merge
  if: steps.metadata.outputs.update-type != 'version-update:semver-major'
  env:
    GITHUB_TOKEN: ${{ secrets.AUTOMATION_TOKEN }}
  run: gh pr merge --auto --merge "$PR_URL"
```

Note also that **dependabot reads `.github/dependabot.yml` from the default branch**, not from
`target-branch`. An ignore rule added only on `dev` has no effect.

## 8. `nixConfig` does not propagate

A flake's `nixConfig.extra-substituters` applies **only when that flake is the top-level flake**. A
downstream flake consuming it as an input does not inherit its substituters — there is no mechanism,
and it would be a supply-chain hole if there were. It is also ignored entirely for non-trusted
users, which is why CI logs `ignoring untrusted flake configuration setting`.

So a consumer that wants cache hits must list the substituter in its own `nix.settings`, and CI must
get it from `cachix-action`. Keep the `nixConfig` block anyway — it serves interactive trusted users
running `nix run github:icebluerabbit/<name>`.

## 9. Wire it into `nixos`

Declare the input in the module that uses it, never by hand-editing `flake.nix`:

```nix
flake-file.inputs.<name> = {
  url = "github:icebluerabbit/<name>/dev";
  inputs.nixpkgs.follows = "nixpkgs";
};
```

Then `just flake`. Consume `nixosModules.default` — **do not `import` an internal file by path**;
`nixos` does that for `youtarr-flake`'s `modules/settings.nix` and is now coupled to its internal
layout.

Add the repo to `modules/hosts/server/containers/home-assistant/_github-repos.nix` for the GitHub
relay automations. Entity `unique_id`s derive from the numeric repo id and entity ids from the name
slug, so all five relay automations pick it up with no further edits.

## 10. Verify before calling it done

```bash
gh api repos/icebluerabbit/<name>/actions/runs --jq '.workflow_runs[]|"\(.event) \(.head_branch) \(.conclusion)"'
```

Confirm a **push-triggered** run exists, not just `workflow_dispatch`. Dispatch succeeding proves
the workflow is valid; only a push run proves triggers work (see the Actions-enable trap in §2).

The commit author must be `icebluerabbit <314762863+icebluerabbit@users.noreply.github.com>`. No
`Co-Authored-By` or other trailer, ever.
