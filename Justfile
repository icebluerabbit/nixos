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

build: build-server build-desktop

test-desktop: flake
    nh os test .#desktop

test-server: flake
    nh os test .#server

eval-desktop: flake
    nix eval .#nixosConfigurations.desktop.config.system.build.toplevel.drvPath

eval-server: flake
    nix eval .#nixosConfigurations.server.config.system.build.toplevel.drvPath

test: test-server test-desktop

switch-desktop: flake format
    nh os switch .#desktop

switch-server: flake format
    nh os switch . --target-host {{ server }} --hostname server

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

# Same, on every save under modules/infra or packages/topology.
topology-watch file="README.md":
    nix run nixpkgs#watchexec -- \
      -w modules/infra -w packages/topology -e nix,go \
      -- just topology {{file}}

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
