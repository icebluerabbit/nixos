# ssh target for the server host; desktop recipes run locally
server := "zms@10.10.20.2"

default:
    @just --list

_track:
    @git add -N .

build-desktop: flake
    nh os build .#desktop

build-server: flake
    nh os build .#server

build-laptop: flake
    nh darwin build .#laptop

build: build-server build-desktop

test-desktop: flake
    nh os test .#desktop

test-server: flake
    nh os test .#server

eval-desktop: flake
    nix eval .#nixosConfigurations.desktop.config.system.build.toplevel.drvPath

eval-server: flake
    nix eval .#nixosConfigurations.server.config.system.build.toplevel.drvPath

eval-laptop: flake
    nix eval .#darwinConfigurations.laptop.config.system.build.toplevel.drvPath

test: test-server test-desktop

switch-desktop: flake format
    nh os switch .#desktop

switch-server: flake format
    nh os switch . --target-host {{ server }} --hostname server

switch-laptop: flake format
    nh darwin switch .#laptop

switch: infra-apply-homelab switch-server switch-desktop

boot-desktop: flake format
    nh os boot .#desktop

boot-server: flake format
    nh os boot . --target-host {{ server }} --hostname server

# `systemctl status` exits 3 on an inactive unit, which is what a successful
# oneshot looks like once it finishes — so start and status are both allowed to
# fail, and `is-failed` decides the recipe's exit code. Errors are ignored on
# the first two lines so the status output still prints when a backup fails.
backup-desktop:
    -sudo systemctl start restic-projects.service
    -systemctl status restic-projects.service --no-pager -n 20
    ! systemctl is-failed --quiet restic-projects.service

backup-server:
    -ssh {{ server }} sudo systemctl start restic-appdata.service
    -ssh {{ server }} systemctl status restic-appdata.service --no-pager -n 20
    ! ssh {{ server }} systemctl is-failed --quiet restic-appdata.service

backup: backup-server backup-desktop

format: _track
    nix fmt

infra-plan-homelab: flake
    nix run .#homelab.plan

infra-apply-homelab: flake
    nix run .#homelab.apply

vm-desktop: _track
    nix run .#vm-desktop

vm-server: _track
    nix run .#vm-server

make-iso host="desktop" out="/tmp/nixos-installer": _track
    #!/usr/bin/env bash
    OUT="{{out}}"
    if [ -z "$OUT" ]; then OUT="/tmp/nixos-installer"; fi
    mkdir -p "$(dirname "$OUT")"
    nix build .#iso-{{host}} --out-link "$OUT"
    ISO_PATH=$(ls "$OUT"/iso/*.iso | head -n 1)
    echo "ISO created at: $ISO_PATH"

test-iso host="desktop" out="/tmp/nixos-installer" key="": (make-iso host out)
    #!/usr/bin/env bash
    TPM_DIR=$(mktemp -d /tmp/swtpm.XXXXXX)
    DISK_DIR=$(mktemp -d /tmp/nix-disks.XXXXXX)
    LOG_DIR="/tmp/nixos-installer-logs"
    mkdir -p "$LOG_DIR"
    OVMF_BIOS=$(nix build --no-link --print-out-paths nixpkgs#OVMF.fd)/FV/OVMF.fd

    # Use default if out is empty
    OUT_PATH="{{out}}"
    if [ -z "$OUT_PATH" ]; then OUT_PATH="/tmp/nixos-installer"; fi
    ISO_PATH=$(ls "$OUT_PATH"/iso/*.iso | head -n 1)

    SHARE_FLAG=""
    if [ -n "{{key}}" ]; then
        KEY_DIR=$(cd "$(dirname "{{key}}")" && pwd)
        SHARE_FLAG="-virtfs local,path=$KEY_DIR,mount_tag=secrets,security_model=none,readonly=on"
    fi

    nix shell nixpkgs#qemu_kvm nixpkgs#swtpm --command bash -c "
        qemu-img create -f qcow2 \"$DISK_DIR/nvme0.qcow2\" 1G
        qemu-img create -f qcow2 \"$DISK_DIR/nvme1.qcow2\" 1G
        qemu-img create -f qcow2 \"$DISK_DIR/nvme2.qcow2\" 1G
        qemu-img create -f qcow2 \"$DISK_DIR/nvme3.qcow2\" 40G
        swtpm socket --tpmstate dir=$TPM_DIR --ctrl type=unixio,path=$TPM_DIR/swtpm-sock --tpm2 &
        SWTPM_PID=\$!
        qemu-system-x86_64 \
            -enable-kvm -m 8G \
            $SHARE_FLAG \
            -bios $OVMF_BIOS \
            -chardev socket,id=chrtpm,path=$TPM_DIR/swtpm-sock \
            -tpmdev emulator,id=tpm0,chardev=chrtpm \
            -device tpm-tis,tpmdev=tpm0 \
            -cdrom \"$ISO_PATH\" \
            -serial file:\"$LOG_DIR/guest.log\" \
            -drive file=\"$DISK_DIR/nvme0.qcow2\",if=none,id=d0 -device nvme,drive=d0,serial=d0 \
            -drive file=\"$DISK_DIR/nvme1.qcow2\",if=none,id=d1 -device nvme,drive=d1,serial=d1 \
            -drive file=\"$DISK_DIR/nvme2.qcow2\",if=none,id=d2 -device nvme,drive=d2,serial=d2 \
            -drive file=\"$DISK_DIR/nvme3.qcow2\",if=none,id=d3 -device nvme,drive=d3,serial=d3 \
            -net nic -net user,hostfwd=tcp::2222-:22
        kill \$SWTPM_PID
    "
    rm -rf "$TPM_DIR" "$DISK_DIR"

# Regenerate the topology sections of README.md in place. Nix evaluates the
# registries and the host's own networking, the Go renderer turns them into
# markdown, and each view is spliced between its BEGIN/END markers — GitHub has
# no include, so a README that shows this has to contain it.
_topology-dump:
    @nix eval .#nixosConfigurations.server.config --json --apply 'c: { \
        inherit (c) infra; \
        bonds = builtins.mapAttrs (_: b: { inherit (b) interfaces; }) c.networking.bonds; \
        vlans = builtins.mapAttrs (_: v: { inherit (v) id interface; }) c.networking.vlans; \
        bridges = builtins.mapAttrs (_: b: { inherit (b) interfaces; }) c.networking.bridges; \
        containers = builtins.mapAttrs (_: k: { inherit (k) hostBridge localAddress; }) c.containers; \
      }'

topology file="README.md": _track
    @just _topology-dump | nix run .#topology -- --view physical \
        --into {{file}} --marker topology-physical
    @just _topology-dump | nix run .#topology -- --view logical \
        --into {{file}} --marker topology-logical
    @echo "{{file}}"

# Same, on every save under modules/infra or packages/go/topology.
topology-watch file="README.md":
    nix run nixpkgs#watchexec -- \
      -w modules/infra -w packages/go -e nix,go \
      -- just topology {{file}}

# go.work is the registry: a module listed there is linted, tested and covered,
# and one that is not is invisible to the whole gate. Read through `go work
# edit -json` rather than parsed by hand so formatting changes cannot break it.
_go-modules:
    @cd packages/go && go work edit -json | jq -r '.Use[].DiskPath' | sed 's|^\./||'

# The flake attribute that builds a module. Every project is named after its
# binary; lib is the one module producing none, so it carries a prefix that
# would not read like nixpkgs' own `lib`.
_go-attr module:
    @if [ "{{ module }}" = "lib" ]; then echo homelab-go-lib; else echo "{{ module }}"; fi

# GOWORK is off throughout so these behave exactly as the Nix build does, which
# vendors each module separately and would be confused by the workspace file.
go-test:
    #!/usr/bin/env bash
    set -euo pipefail
    cd packages/go
    for m in $(just _go-modules); do
        echo "==> $m"
        (cd "$m" && GOWORK=off go test -race ./...)
    done

go-lint:
    #!/usr/bin/env bash
    set -euo pipefail
    cd packages/go
    for m in $(just _go-modules); do
        echo "==> $m"
        (cd "$m" && GOWORK=off golangci-lint run --config ../.golangci.yml ./...)
    done

# Run every //go:generate directive in every module.
#
# Go deliberately does not run these as part of a build, so generated code is
# only as fresh as the last person to remember. This is that reminder in one
# command; the generated output is committed, because flakes ignore untracked
# files and Nix has to be able to see it.
#
# GOWORK stays ON here, unlike the rest: a generator imports the module it is
# generating for, and with the workspace off it would resolve `replace` targets
# against a vendor tree that does not yet contain the code being written.
go-generate:
    #!/usr/bin/env bash
    set -euo pipefail
    cd packages/go
    for m in $(just _go-modules); do
        echo "==> $m"
        (cd "$m" && go generate ./...)
    done

# Coverage floors every package except the `main` ones. A cmd/<name>/main.go
# holds no logic by construction, so covering it would measure nothing — but
# everything it delegates to is gated.
go-cover threshold="80":
    #!/usr/bin/env bash
    set -euo pipefail
    cd packages/go
    status=0
    for m in $(just _go-modules); do
        # go list -json rather than -f: just would eat a Go template's braces.
        pkgs=$(cd "$m" && GOWORK=off go list -json ./... | jq -r 'select(.Name != "main") | .ImportPath')
        if [ -z "$pkgs" ]; then
            echo "==> $m: no non-main packages"
            continue
        fi
        profile=$(mktemp)
        # shellcheck disable=SC2086
        (cd "$m" && GOWORK=off go test -covermode=atomic -coverprofile="$profile" $pkgs >/dev/null)
        pct=$(cd "$m" && GOWORK=off go tool cover -func="$profile" | awk '/^total:/ {print $3}' | tr -d '%')
        rm -f "$profile"
        echo "==> $m: ${pct}% (minimum {{ threshold }}%)"
        if awk -v p="$pct" -v t="{{ threshold }}" 'BEGIN { exit !(p + 0 < t + 0) }'; then
            echo "    below threshold"
            status=1
        fi
    done
    exit "$status"

# Rewrite packages/go/vendor-hashes.nix from what Nix actually computes.
#
# Nix will only report a vendor hash when the one on record is wrong, so each
# entry is set to a known-wrong value first and the correct hash is read out of
# the resulting mismatch. A module with no external dependencies reports none,
# and is recorded as null.
go-hashes: _track
    #!/usr/bin/env bash
    set -euo pipefail
    wrong="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
    hashes=packages/go/vendor-hashes.nix
    for m in $(just _go-modules); do
        attr=$(just _go-attr "$m")
        # Neutralise this module's entry so the build is forced to disagree.
        sed -i -E "s|^(  ${m} = ).*;|\\1\"${wrong}\";|" "$hashes"
        # The build is *expected* to fail — that failure is where the hash
        # comes from — so its status is discarded rather than tripping set -e.
        out=$(nix build ".#${attr}" --no-link 2>&1 || true)
        got=$(printf '%s\n' "$out" | awk '/got:/ {print $2; exit}')
        if [ -z "$got" ]; then
            echo "==> $m: no external dependencies"
            sed -i -E "s|^(  ${m} = ).*;|\\1null;|" "$hashes"
            continue
        fi
        echo "==> $m: $got"
        sed -i -E "s|^(  ${m} = ).*;|\\1\"${got}\";|" "$hashes"
    done
    nix fmt >/dev/null

# Bump Go dependencies across every module, then refresh the vendor hashes so
# the tree is buildable again in one step.
go-update: && go-hashes
    #!/usr/bin/env bash
    set -euo pipefail
    cd packages/go
    for m in $(just _go-modules); do
        echo "==> $m"
        (cd "$m" && GOWORK=off go get -u ./... && GOWORK=off go mod tidy)
    done

go: go-lint go-test go-cover

flake: _track
    nix run .#write-flake

update:
    nix flake update

update-den:
    nix flake update den

sops-user-secrets:
    nix run nixpkgs#sops -- secrets/user-secrets.yaml

sops-host-secrets:
    sudo SOPS_AGE_KEY_FILE=/var/lib/sops-nix/key.txt nix run nixpkgs#sops -- secrets/host-secrets.yaml

clean:
    sudo nix-collect-garbage -d

gc:
    nh clean all
