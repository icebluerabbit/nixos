# vault — on-demand LUKS2 volume on the spare 990 PRO; never mounted at boot
_: {
  den.aspects.hosts._.desktop._.vault = {
    aspect-name = "hosts.desktop.vault";

    nixos =
      { pkgs, ... }:
      let
        # Whole-disk LUKS2 on the spare 990 PRO, keyed by the LUKS2 header label
        # rather than the drive serial so the vault follows its data across a
        # slot move or a drive swap. The inner btrfs is deliberately left
        # unlabelled: udev's by-label rule matches ID_FS_USAGE=crypto too, so a
        # second `vault` label would collide on this very symlink once unlocked.
        #
        # Not a disko entry on purpose: disko describes an install-time wipe, and
        # a second `disk.*` stanza means anyone re-running it reformats the vault
        # along with the root disk.
        #
        # No TPM2 enrolment either — a TPM unseals unattended, which is exactly
        # the property this volume exists to avoid. Passphrase, or
        # `systemd-cryptenroll --fido2-device=auto` for a touch-to-unlock token.
        device = "/dev/disk/by-label/vault";

        # packages/vault takes the volume as flags so it stays host-agnostic;
        # this host pins its one vault here rather than baking the location into
        # the package. Flags are prepended, so `sudo vault open` needs no
        # arguments while the flags stay overridable.
        #
        # --app is wired here for the same reason: packages/ is callPackage'd
        # against plain nixpkgs, so one homelab package cannot reference another
        # by name (see modules/overlays.nix) — the host is where the two meet.
        vault = pkgs.symlinkJoin {
          name = "vault-desktop";
          paths = [ pkgs.homelab.vault ];
          nativeBuildInputs = [ pkgs.makeWrapper ];
          postBuild = ''
            wrapProgram $out/bin/vault \
              --add-flags "--device ${device} --name vault --mountpoint /vault" \
              --add-flags "--app ${pkgs.homelab.vault-app}"
          '';
        };
      in
      {
        environment.systemPackages = [ vault ];

        fileSystems."/vault" = {
          device = "/dev/mapper/vault";
          fsType = "btrfs";
          options = [
            # `noauto` and deliberately NOT `x-systemd.automount`: an automount
            # unit would fire an unlock attempt on any stray `ls /vault`, which
            # inverts the whole point of an on-demand volume.
            "noauto"
            "subvol=vault"
            "compress=zstd:3"
            "noatime"
            "ssd"
            "discard=async"
            "space_cache=v2"
          ];
        };

        systemd.tmpfiles.rules = [
          # Mountpoint owned by zms and 0700 — while locked this is a bare
          # directory on the root filesystem, and it should not be a
          # world-readable landing spot for anything written to it by mistake.
          "d /vault 0700 zms users -"
        ];
      };
  };
}
