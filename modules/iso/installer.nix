# installer — automated NixOS host installation and secrets bootstrap scripts
{ inputs, ... }:
{
  den.default.nixos =
    {
      pkgs,
      lib,
      config,
      ...
    }:
    let
      hostName = config.networking.hostName;
      users = lib.filterAttrs (_: u: u.isNormalUser) config.users.users;
      userData = lib.mapAttrsToList (name: u: {
        inherit name;
        inherit (u) uid;
        gid = config.users.groups.${u.group}.gid;
      }) users;

      logger = pkgs.writeShellApplication {
        name = "logger";
        runtimeInputs = [ pkgs.coreutils ];
        text = ''
          level=$1
          shift
          timestamp=$(date "+%Y-%m-%d %H:%M:%S")
          case "$level" in
            info)    echo -e "\033[1;34m[INFO]\033[0m    \033[2m$timestamp\033[0m $*" ;;
            success) echo -e "\033[1;32m[SUCCESS]\033[0m \033[2m$timestamp\033[0m $*" ;;
            warn)    echo -e "\033[1;33m[WARN]\033[0m    \033[2m$timestamp\033[0m $*" ;;
            debug)   echo -e "\033[1;30m[DEBUG]\033[0m   \033[2m$timestamp\033[0m $*" ;;
            *)       echo "[$level] $timestamp $*" ;;
          esac
        '';
      };

      nixos-repo = pkgs.stdenv.mkDerivation {
        name = "nixos-repo";
        src = lib.cleanSource inputs.self;
        installPhase = "cp -r . $out";
      };

      tpm-enroll =
        let
          blkid = lib.getExe' pkgs.util-linux "blkid";
          cryptenroll = lib.getExe' pkgs.systemd "systemd-cryptenroll";
          udevadm = lib.getExe' pkgs.systemd "udevadm";
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "tpm-enroll";
          runtimeInputs = [
            pkgs.util-linux
            pkgs.systemd
          ];
          text = ''
            ${logger-exe} info "Enrolling TPM2..."
            sudo ${udevadm} settle
            LUKS_DEVS=$(sudo ${blkid} -t TYPE=crypto_LUKS -o device) || ${logger-exe} warn "No LUKS devices found"

            for dev in $LUKS_DEVS; do
                if [ -f /tmp/luks.key ]; then
                    PASSWORD=$(sudo cat /tmp/luks.key)
                    export PASSWORD
                fi

                ${logger-exe} debug "Enrolling TPM2 with PIN for $dev..."
                # shellcheck disable=SC2024
                sudo -E ${cryptenroll} \
                    --tpm2-device=auto \
                    --tpm2-pcrs=0+2 \
                    --tpm2-with-pin=yes \
                    "$dev" < /dev/tty

                ${logger-exe} success "Successfully enrolled TPM2 for $dev"

                ${logger-exe} debug "Wiping password slots for $dev..."
                sudo -E ${cryptenroll} --wipe-slot=password "$dev"
                ${logger-exe} success "Wiped password slots for $dev"

                unset PASSWORD
            done
          '';
        };

      sops-host-bootstrap =
        let
          mkdir = lib.getExe' pkgs.coreutils "mkdir";
          chmod = lib.getExe' pkgs.coreutils "chmod";
          age-keygen = lib.getExe' pkgs.age "age-keygen";
          sed = lib.getExe pkgs.gnused;
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "sops-host-bootstrap";
          runtimeInputs = [
            pkgs.coreutils
            pkgs.age
            pkgs.gnused
          ];
          text = ''
            ${logger-exe} debug "Generating Host Key in /mnt..."
            sudo ${mkdir} -p "/mnt/var/lib/sops-nix"
            sudo ${age-keygen} -o "/mnt/var/lib/sops-nix/key.txt" 2>/dev/null
            NEW_HOST_PUB=$(sudo ${age-keygen} -y "/mnt/var/lib/sops-nix/key.txt")
            sudo ${chmod} 600 "/mnt/var/lib/sops-nix/key.txt"
            sudo ${sed} -i "s|&${hostName} age.*|\\&${hostName} $NEW_HOST_PUB|" "/mnt/src/nixos/.sops.yaml"
          '';
        };

      sops-host-updatekeys =
        let
          sops = lib.getExe pkgs.sops;
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "sops-host-updatekeys";
          runtimeInputs = [ pkgs.sops ];
          text = ''
            ${logger-exe} debug "Updating host secret keys..."
            SOPS_AGE_KEY="$1" sudo -E ${sops} --config /mnt/src/nixos/.sops.yaml updatekeys /mnt/src/nixos/secrets/host-secrets.yaml -y
          '';
        };

      sops-user-bootstrap =
        let
          mkdir = lib.getExe' pkgs.coreutils "mkdir";
          chmod = lib.getExe' pkgs.coreutils "chmod";
          chown = lib.getExe' pkgs.coreutils "chown";
          age-keygen = lib.getExe' pkgs.age "age-keygen";
          sed = lib.getExe pkgs.gnused;
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "sops-user-bootstrap";
          runtimeInputs = [
            pkgs.coreutils
            pkgs.age
            pkgs.gnused
          ];
          text = ''
            while IFS=: read -r username uid gid; do
                [ -z "$username" ] && continue
                ${logger-exe} debug "Generating keys for $username (UID: $uid, GID: $gid)..."
                USER_AGE_DIR="/mnt/home/$username/.config/sops/age"
                sudo ${mkdir} -p "$USER_AGE_DIR"
                sudo ${age-keygen} -o "$USER_AGE_DIR/keys.txt" 2>/dev/null
                NEW_USER_PUB=$(sudo ${age-keygen} -y "$USER_AGE_DIR/keys.txt")
                sudo ${sed} -i "s|&$username age.*|\\&$username $NEW_USER_PUB|" "/mnt/src/nixos/.sops.yaml"
                sudo ${chown} -R "$uid":"$gid" "/mnt/home/$username"
                sudo ${chmod} 600 "$USER_AGE_DIR/keys.txt"
            done <<EOF
            ${lib.concatMapStrings (u: "${u.name}:${toString u.uid}:${toString u.gid}\n") userData}
            EOF
          '';
        };

      sops-user-updatekeys =
        let
          sops = lib.getExe pkgs.sops;
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "sops-user-updatekeys";
          runtimeInputs = [ pkgs.sops ];
          text = ''
            ${logger-exe} debug "Updating user secret keys..."
            SOPS_AGE_KEY="$1" sudo -E ${sops} --config /mnt/src/nixos/.sops.yaml updatekeys /mnt/src/nixos/secrets/user-secrets.yaml -y
          '';
        };

      disko-run =
        let
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "disko-run";
          text = ''
            ${logger-exe} debug "Running Disko for ${hostName}..."
            sudo ${config.system.build.diskoScript}
          '';
        };

      repo-copy =
        let
          mkdir = lib.getExe' pkgs.coreutils "mkdir";
          cp = lib.getExe' pkgs.coreutils "cp";
          chown = lib.getExe' pkgs.coreutils "chown";
          chmod = lib.getExe' pkgs.coreutils "chmod";
          logger-exe = lib.getExe logger;
          nixosConfigGid = toString config.users.groups.nixos-config.gid;
        in
        pkgs.writeShellApplication {
          name = "repo-copy";
          runtimeInputs = [ pkgs.coreutils ];
          text = ''
            ${logger-exe} debug "Copying repository to /mnt/src/nixos..."
            sudo ${mkdir} -p "/mnt/src/nixos"
            sudo ${cp} -r "${nixos-repo}"/. "/mnt/src/nixos/"
            sudo ${chown} -R root:${nixosConfigGid} "/mnt/src/nixos"
            sudo ${chmod} -R g+w "/mnt/src/nixos"
          '';
        };

      nixos-install-run =
        let
          nixos-install = lib.getExe' pkgs.nixos-install-tools "nixos-install";
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "nixos-install-run";
          runtimeInputs = [ pkgs.nixos-install-tools ];
          text = ''
            ${logger-exe} debug "Running nixos-install for ${hostName}..."
            sudo ${nixos-install} --flake "/mnt/src/nixos#${hostName}" --no-root-password
          '';
        };
    in
    {
      system.build.install-host =
        let
          disko-run-exe = lib.getExe disko-run;
          repo-copy-exe = lib.getExe repo-copy;
          sops-host-bootstrap-exe = lib.getExe sops-host-bootstrap;
          sops-host-updatekeys-exe = lib.getExe sops-host-updatekeys;
          sops-user-bootstrap-exe = lib.getExe sops-user-bootstrap;
          sops-user-updatekeys-exe = lib.getExe sops-user-updatekeys;
          tpm-enroll-exe = lib.getExe tpm-enroll;
          nixos-install-run-exe = lib.getExe nixos-install-run;
          openssl = lib.getExe pkgs.openssl;
          logger-exe = lib.getExe logger;
        in
        pkgs.writeShellApplication {
          name = "install-host";
          runtimeInputs = [
            disko-run
            repo-copy
            sops-host-bootstrap
            sops-host-updatekeys
            sops-user-bootstrap
            sops-user-updatekeys
            tpm-enroll
            nixos-install-run
            pkgs.openssl
          ];
          text = ''
            ${logger-exe} info "Starting installation for ${hostName}..."

            ${openssl} rand -base64 32 > /tmp/luks.key
            chmod 600 /tmp/luks.key

            ${logger-exe} info "Formatting disks..."
            ${disko-run-exe}
            ${logger-exe} success "Disks formatted successfully"

            ${logger-exe} info "Copying repository..."
            ${repo-copy-exe}
            ${logger-exe} success "Repository copied"

            ${logger-exe} info "Preparing secrets..."
            KEY_CONTENT=$(sudo cat "$1")

            ${logger-exe} info "Host secrets..."
            ${sops-host-bootstrap-exe}
            ${sops-host-updatekeys-exe} "$KEY_CONTENT"
            ${logger-exe} success "Host secrets configured"

            ${logger-exe} info "User secrets..."
            ${sops-user-bootstrap-exe}
            ${sops-user-updatekeys-exe} "$KEY_CONTENT"
            ${logger-exe} success "User secrets configured"

            ${logger-exe} info "Enrolling TPM2..."
            ${tpm-enroll-exe}
            ${logger-exe} success "TPM2 enrollment finished"

            ${logger-exe} info "Running system installation..."
            ${nixos-install-run-exe}
            ${logger-exe} success "System installation completed"

            rm -f /tmp/luks.key
            ${logger-exe} success "Installation finished successfully"
          '';
        };
    };
}
