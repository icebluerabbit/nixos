# iso — custom bootable NixOS installation media, one image per host
{
  inputs,
  self,
  lib,
  ...
}:
{
  perSystem =
    { pkgs, system, ... }:
    let
      # Hosts installable on this architecture.
      compatibleHosts = lib.filterAttrs (
        _: cfg: cfg.config.nixpkgs.hostPlatform.system == system
      ) self.nixosConfigurations;

      # A minimal installer ISO carrying exactly one target host: its
      # pre-seeded system closure (offline install) and its install script.
      mkInstallerIso =
        name: hostCfg:
        inputs.nixpkgs.lib.nixosSystem {
          modules = [
            "${inputs.nixpkgs}/nixos/modules/installer/cd-dvd/installation-cd-minimal.nix"
            {
              nixpkgs.hostPlatform = system;

              boot = {
                # Fix: boot.zfs.forceImportRoot warning
                zfs.forceImportRoot = false;
                tmp = {
                  useTmpfs = true;
                  tmpfsSize = "90%";
                };
              };

              isoImage.squashfsCompression = "lz4";
              nix.settings.experimental-features = [
                "nix-command"
                "flakes"
              ];

              networking.networkmanager.enable = true;
              networking.wireless.enable = true;

              services.openssh = {
                enable = true;
                settings.PermitRootLogin = "prohibit-password";
              };

              services.getty.autologinUser = "nixos";

              users.users.nixos = {
                isNormalUser = true;
                extraGroups = [
                  "wheel"
                  "networkmanager"
                  "dialout"
                ];
                # Fix: Multiple password options warning
                initialHashedPassword = lib.mkForce null;
                password = "nixos";
              };

              environment.systemPackages =
                let
                  installScript = pkgs.writeShellScriptBin "install-${name}.sh" ''
                    exec ${lib.getExe hostCfg.config.system.build.install-host} "$@"
                  '';

                  mountSecrets = pkgs.writeShellScriptBin "mount-secrets" ''
                    sudo mkdir -p /tmp/secrets
                    sudo mount -t 9p -o trans=virtio secrets /tmp/secrets
                    echo "Secrets mounted at /tmp/secrets"
                  '';
                in
                [
                  installScript
                  mountSecrets
                  pkgs.git
                  pkgs.vim
                ];

              # Pre-package only this host's system closure.
              isoImage.storeContents = [ hostCfg.config.system.build.toplevel ];

              programs.bash.loginShellInit = ''
                # Auto-setup for VM testing
                if ${pkgs.systemd}/bin/systemd-detect-virt -q; then
                    # Auto-redirect logs if we're on the physical console (tty1)
                    if [ -e /dev/ttyS0 ] && [ "$(tty 2>/dev/null)" != "/dev/ttyS0" ]; then
                        exec > >(sudo tee -a /dev/ttyS0) 2>&1
                    fi
                    mount-secrets 2>/dev/null || true
                fi

                clear
                echo "Modular NixOS ISO — ${name}"
                echo "========================"
                echo "Install script: install-${name}.sh"
                echo ""
                if [ -d /tmp/secrets ] && mountpoint -q /tmp/secrets; then
                    echo "VM Status: Logs mirrored to guest.log, Secrets mounted at /tmp/secrets"
                    install-${name}.sh /tmp/secrets/root.txt
                else
                    echo "VM Status: Logs mirrored to guest.log"
                fi
              '';
            }
          ];
        };
    in
    # Installer images are NixOS systems; only emit them on Linux systems.
    # (Without the guard, an aarch64-darwin host would try to build a bogus
    # darwin NixOS installer.)
    lib.optionalAttrs (lib.hasSuffix "-linux" system) {
      packages = lib.mapAttrs' (
        name: hostCfg:
        lib.nameValuePair "iso-${name}" (mkInstallerIso name hostCfg).config.system.build.isoImage
      ) compatibleHosts;
    };
}
